// Code generated by github.com/whyrusleeping/cbor-gen. DO NOT EDIT.

package skiplist

import (
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	cbg "github.com/whyrusleeping/cbor-gen"
	xerrors "golang.org/x/xerrors"
)

var _ = xerrors.Errorf

func (t *Node) MarshalCBOR(w io.Writer) error {
	if t == nil {
		_, err := w.Write(cbg.CborNull)
		return err
	}
	if _, err := w.Write([]byte{131}); err != nil {
		return err
	}

	// t.Index (uint64) (uint64)

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajUnsignedInt, uint64(t.Index))); err != nil {
		return err
	}

	// t.Links ([]cid.Cid) (slice)
	if len(t.Links) > cbg.MaxLength {
		return xerrors.Errorf("Slice value in field t.Links was too long")
	}

	if _, err := w.Write(cbg.CborEncodeMajorType(cbg.MajArray, uint64(len(t.Links)))); err != nil {
		return err
	}
	for _, v := range t.Links {
		if err := cbg.WriteCid(w, v); err != nil {
			return xerrors.Errorf("failed writing cid field t.Links: %w", err)
		}
	}

	// t.Value (typegen.Deferred) (struct)
	if err := t.Value.MarshalCBOR(w); err != nil {
		return err
	}
	return nil
}

func (t *Node) UnmarshalCBOR(r io.Reader) error {
	br := cbg.GetPeeker(r)

	maj, extra, err := cbg.CborReadHeader(br)
	if err != nil {
		return err
	}
	if maj != cbg.MajArray {
		return fmt.Errorf("cbor input should be of type array")
	}

	if extra != 3 {
		return fmt.Errorf("cbor input had wrong number of fields")
	}

	// t.Index (uint64) (uint64)

	{

		maj, extra, err = cbg.CborReadHeader(br)
		if err != nil {
			return err
		}
		if maj != cbg.MajUnsignedInt {
			return fmt.Errorf("wrong type for uint64 field")
		}
		t.Index = uint64(extra)

	}
	// t.Links ([]cid.Cid) (slice)

	maj, extra, err = cbg.CborReadHeader(br)
	if err != nil {
		return err
	}

	if extra > cbg.MaxLength {
		return fmt.Errorf("t.Links: array too large (%d)", extra)
	}

	if maj != cbg.MajArray {
		return fmt.Errorf("expected cbor array")
	}

	if extra > 0 {
		t.Links = make([]cid.Cid, extra)
	}

	for i := 0; i < int(extra); i++ {

		c, err := cbg.ReadCid(br)
		if err != nil {
			return xerrors.Errorf("reading cid field t.Links failed: %w", err)
		}
		t.Links[i] = c
	}

	// t.Value (typegen.Deferred) (struct)

	{

		pb, err := br.PeekByte()
		if err != nil {
			return err
		}
		if pb == cbg.CborNull[0] {
			var nbuf [1]byte
			if _, err := br.Read(nbuf[:]); err != nil {
				return err
			}
		} else {
			t.Value = new(cbg.Deferred)
			if err := t.Value.UnmarshalCBOR(br); err != nil {
				return xerrors.Errorf("unmarshaling t.Value pointer: %w", err)
			}
		}

	}
	return nil
}
