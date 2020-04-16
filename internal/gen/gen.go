package main

import (
	"github.com/Kubuxu/go-skiplist-ipld"
	cbg "github.com/whyrusleeping/cbor-gen"
)

func main() {
	// FIXME this will not generate the correct code, leave the cbor_gen.go file untouched.
	if err := cbg.WriteTupleEncodersToFile("cbor_gen.go", "skiplist", skiplist.Node{}); err != nil {
		panic(err)
	}
}
