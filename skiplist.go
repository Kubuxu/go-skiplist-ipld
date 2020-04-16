package skiplist

import (
	"bytes"
	"context"
	"errors"
	"math/bits"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	mh "github.com/multiformats/go-multihash"
	cbg "github.com/whyrusleeping/cbor-gen"
	"golang.org/x/xerrors"
)

var log = logging.Logger("skiplist")

var (
	ErrEmptyList  = errors.New("skiplist is empty")
	ErrEmptyValue = errors.New("value is empty")
	ErrNotFound   = errors.New("index not found")
)

type Node struct {
	Index uint64
	Links []cid.Cid
	Value *cbg.Deferred
}

type cacheVal struct {
	n Node
	c cid.Cid
}

func (n Node) computeCid() (cid.Cid, error) {
	p := cid.Prefix{
		Version:  1,
		MhType:   uint64(mh.BLAKE2B_MIN + 31),
		MhLength: -1,
		Codec:    cid.DagCBOR,
	}
	buf := new(bytes.Buffer)
	n.MarshalCBOR(buf)
	return p.Sum(buf.Bytes())
}

type Head struct {
	node *Node
	c    cid.Cid

	store cbor.IpldStore
	cache *lru.ARCCache
}

func New(bs cbor.IpldStore) *Head {
	cache, err := lru.NewARC(128)
	if err != nil {
		panic(err)
	}
	return &Head{
		store: bs,
		c:     cid.Undef,
		cache: cache,
	}
}

func Load(ctx context.Context, bs cbor.IpldStore, c cid.Cid) (*Head, error) {
	var n Node
	if err := bs.Get(ctx, c, &n); err != nil {
		return nil, xerrors.Errorf("could not load head node: %w", err)
	}

	cache, err := lru.NewARC(128)
	if err != nil {
		panic(err)
	}
	cache.Add(n.Index, cacheVal{n, c})

	return &Head{
		node:  &n,
		c:     c,
		store: bs,
		cache: cache,
	}, nil
}

func (h *Head) Index() (uint64, error) {
	if h.node == nil {
		return 0, ErrEmptyList
	}
	return h.node.Index, nil
}

func (h *Head) Get(ctx context.Context, i uint64, out interface{}) error {
	if h.node == nil {
		return xerrors.Errorf("could not get at %d: %w", i, ErrEmptyList)
	}
	if h.node.Index == i {
		return decode(h.node.Value, out)
	}

	n, _, err := h.lookup(ctx, *h.node, i)
	if err != nil {
		return xerrors.Errorf("looking up: %w", err)
	}

	if n.Index == i {
		return decode(n.Value, out)
	}

	return ErrNotFound
}

func (h *Head) lookup(ctx context.Context, start Node, i uint64) (*Node, cid.Cid, error) {
	if i == start.Index {
		c, err := start.computeCid()
		if err != nil {
			return nil, cid.Undef, xerrors.Errorf("computing cid: %w", err)
		}
		return &start, c, nil
	}
	if i > start.Index {
		return nil, cid.Undef,
			xerrors.Errorf("index (%d) higher than node index (%d): %w", i, start.Index, ErrNotFound)
	}

	n := start
	if v, ok := h.cache.Get(i); ok {
		cv := v.(cacheVal)
		return &cv.n, cv.c, nil
	}

	var c cid.Cid
	for n.Index > i {
		if len(n.Links) == 0 {
			return nil, cid.Undef, xerrors.Errorf("node doesn't have links")
		}
		dist := n.Index - i
		order := bits.Len64(dist) - 1
		if order > len(n.Links)-1 {
			order = len(n.Links) - 1
		}

		if v, ok := h.cache.Get(n.Index - 1<<order); ok {
			cv := v.(cacheVal)
			n = cv.n
			c = cv.c
		} else {

			c = n.Links[order]
			if err := h.store.Get(ctx, c, &n); err != nil {
				return nil, cid.Undef, xerrors.Errorf("could not load node while walking: %w", err)
			}
			h.cache.Add(n.Index, cacheVal{n, c})
		}
	}

	return &n, c, nil
}

func (h *Head) Append(ctx context.Context, val interface{}) error {
	if h.node == nil {
		b, err := encode(val)
		if err != nil {
			return xerrors.Errorf("appending: %w", err)
		}
		h.node = &Node{
			Index: 0,
			Links: nil,
			Value: &cbg.Deferred{Raw: b},
		}
		c, err := h.store.Put(ctx, h.node)
		if err != nil {
			return xerrors.Errorf("storing head: %w", err)
		}
		h.c = c
		h.cache.Add(h.node.Index, cacheVal{*h.node, c})
		return nil
	}

	b, err := encode(val)
	if err != nil {
		return xerrors.Errorf("encoding val: %w", err)
	}

	ix := h.node.Index + 1
	order := bits.TrailingZeros64(ix)
	links := make([]cid.Cid, 1+order)
	links[0] = h.c
	n := h.node
	for i := 1; i <= order; i++ {
		var c cid.Cid
		n, c, err = h.lookup(ctx, *n, ix-1<<i)
		if err != nil {
			return xerrors.Errorf("creating skiplinks: %w", err)
		}
		links[i] = c
	}

	newNode := &Node{
		Index: ix,
		Links: links,
		Value: &cbg.Deferred{Raw: b},
	}

	h.node = newNode

	c, err := h.store.Put(ctx, h.node)
	if err != nil {
		return xerrors.Errorf("storing head: %w", err)
	}
	h.c = c
	h.cache.Add(h.node.Index, cacheVal{*h.node, c})

	return nil
}

func (h *Head) Cid() cid.Cid {
	return h.c
}

func encode(val interface{}) ([]byte, error) {
	var b []byte
	if m, ok := val.(cbg.CBORMarshaler); ok {
		buf := new(bytes.Buffer)
		if err := m.MarshalCBOR(buf); err != nil {
			return nil, xerrors.Errorf("cbg.CBORMarshaler: %w", err)
		}
		b = buf.Bytes()
	} else {
		var err error
		b, err = cbor.DumpObject(val)
		if err != nil {
			return nil, xerrors.Errorf("cbor.DumpObject: %w", err)
		}
	}
	return b, nil
}

func decode(d *cbg.Deferred, out interface{}) error {
	if d == nil {
		return xerrors.Errorf("decoding: %w", ErrEmptyValue)
	}

	switch tout := out.(type) {
	case cbg.CBORUnmarshaler:
		if err := tout.UnmarshalCBOR(bytes.NewReader(d.Raw)); err != nil {
			return xerrors.Errorf("out.UnmarshalCBOR: %w", err)
		}
	default:
		if err := cbor.DecodeInto(d.Raw, out); err != nil {
			return xerrors.Errorf("cbor.DecodeInto: %w", err)
		}
	}

	return nil

}
