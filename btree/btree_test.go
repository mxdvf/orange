package btree

import (
	"testing"
)

func TestSimple(t *testing.T) {
	n := NewLeafNode(2)
	n.insertKvPair([]byte("你"), []byte("mehul"))
	n.insertKvPair([]byte("kacky"), []byte("mehul"))
	n.debugPrint()
}
