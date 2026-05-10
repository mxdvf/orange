package btree

import (
	"testing"
)

func TestNKeys(t *testing.T) {
	n := NewLeafNode()

	k := n.getNKeys()
	if k != 0 {
		t.Fatal("nkeys should have been 0")
	}

	n.incrementNKeys()
	k = n.getNKeys()
	if k != 1 {
		t.Fatal("nkeys should have been 1")
	}
}

func TestLeafNodeInsert1(t *testing.T) {
	n := NewLeafNode()
	n.Insert([]byte("kacky"), []byte("mehul"))
	n.Insert([]byte("A"), []byte("Z"))
	n.Insert([]byte("z"), []byte("ain't no way"))
	debugPrint(n, 50)
}
