package util

import (
	"hash"

	"github.com/davecgh/go-spew/spew"
)

func DeepHashObject(hasher hash.Hash, obj any) {
	hasher.Reset()
	p := spew.ConfigState{
		DisableMethods: true,
		Indent:         " ",
		SortKeys:       true,
		SpewKeys:       true,
	}

	_, _ = p.Fprintf(hasher, "%#v", obj)
}
