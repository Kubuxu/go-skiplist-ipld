package skiplist

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"testing"

	block "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"
	assert "github.com/stretchr/testify/assert"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

func init() {
	logging.SetLogLevel("*", "warn")
}

type mockBlocks struct {
	data map[cid.Cid]block.Block
	gets uint64
}

func newMockBlocks() *mockBlocks {
	return &mockBlocks{data: make(map[cid.Cid]block.Block)}
}

func (mb *mockBlocks) Get(c cid.Cid) (block.Block, error) {
	mb.gets++
	d, ok := mb.data[c]
	if ok {
		return d, nil
	}
	return nil, xerrors.Errorf("could not find: %v: %w", c, fmt.Errorf("Not Found"))
}

func (mb *mockBlocks) Put(b block.Block) error {
	mb.data[b.Cid()] = b
	return nil
}

func newBS() (cbor.IpldStore, *mockBlocks) {
	mb := newMockBlocks()
	return cbor.NewCborStore(mb), mb
}

func assertAppend(t *testing.T, h *Head, val interface{}) {
	t.Helper()
	err := h.Append(context.Background(), val)
	assert.NoError(t, err)
}

func assertGet(t *testing.T, h *Head, i uint64, val string) {
	t.Helper()
	var out string
	err := h.Get(context.Background(), i, &out)
	assert.NoError(t, err)
	assert.Equal(t, val, out)
}

func assertIndex(t *testing.T, head *Head, index uint64) {
	t.Helper()
	ix, err := head.Index()
	assert.NoError(t, err)
	assert.Equal(t, index, ix)
}

func TestBasic(t *testing.T) {
	bs, _ := newBS()
	ctx := context.Background()
	l := New(bs)

	err := l.Get(ctx, 0, nil)
	assert.True(t, errors.Is(err, ErrEmptyList), "errors should be: %+v, is: %+v", ErrEmptyList, err)

	assertAppend(t, l, "zero")
	assertGet(t, l, 0, "zero")
	assertIndex(t, l, 0)

	assertAppend(t, l, "one")
	assertIndex(t, l, 1)
	assertAppend(t, l, "two")
	assertIndex(t, l, 2)

	assertGet(t, l, 0, "zero")
	assertGet(t, l, 1, "one")
	assertGet(t, l, 2, "two")
	assertGet(t, l, 0, "zero")
}

func TestBig(t *testing.T) {
	const N = 10000
	bs, _ := newBS()
	ctx := context.Background()
	l := New(bs)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < N; i++ {
		err := l.Append(ctx, i)
		assert.NoError(t, err)
	}
	c := l.Cid()
	l, err := Load(ctx, bs, c)
	assert.NoError(t, err)

	for i := 0; i < 10*N; i++ {
		var res int
		idx := int(rng.Int63n(N))
		err := l.Get(ctx, uint64(idx), &res)
		assert.NoError(t, err)
		assert.Equal(t, idx, res)
	}
}

var R uint64

func BenchmarkAppend(b *testing.B) {
	ctx := context.Background()

	var benches = []int{1, 1000, 10000, 100000}
	for _, M := range benches {
		M := M
		b.Run(fmt.Sprintf("ex-size-%d", M), func(b *testing.B) {
			bs, ms := newBS()
			l := New(bs)
			for i := 0; i < M; i++ {
				err := l.Append(ctx, &TVal{1000})
				if err != nil {
					panic(err)
				}
			}

			c := l.Cid()
			var getC float64
			const N = 100
			b.ResetTimer()

			var r uint64
			for i := 0; i < b.N/N; i++ {
				l, err := Load(ctx, bs, c)
				if err != nil {
					panic(err)
				}
				ms.gets = 0
				for j := uint64(0); j < N; j++ {
					err := l.Append(ctx, &TVal{1000})
					if err != nil {
						panic(err)
					}
				}
				x, _ := l.Index()
				r += x
				getC += float64(ms.gets)
			}
			b.N = (b.N / N) * N
			R += r
			b.ReportMetric(getC/float64(b.N), "gets/op")
		})
	}
}

func BenchmarkGet(b *testing.B) {
	var benches = []uint64{1000, 10000, 100000, 1000000}
	for _, M := range benches {
		M := M
		b.Run(fmt.Sprintf("size-%d", M), func(b *testing.B) {
			bs, mb := newBS()
			ctx := context.Background()
			l := New(bs)
			for i := uint64(0); i < M; i++ {
				l.Append(ctx, &TVal{1993})
			}

			rng := rand.New(rand.NewSource(42))
			b.ResetTimer()

			var r uint64
			for i := 0; i < b.N; i++ {
				var x uint64
				l.Get(ctx, uint64(rng.Int63n(int64(M))), &x)
				r += x
			}
			R += r
			b.ReportMetric(float64(mb.gets)/float64(b.N), "gets/op")
		})
	}
}

type TVal struct {
	i uint64
}

func (t *TVal) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{128}); err != nil {
		return err
	}
	return nil
}

func (t *TVal) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 0 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	return nil
}
