package btree

import (
	"fmt"
	"testing"
)

func TestNodeNKeys(t *testing.T) {
	buf := make([]byte, 4096)
	n := NewNode(buf)

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

func TestNodeLeafNodeInsert1(t *testing.T) {
	buf := make([]byte, 4096)
	n := NewNode(buf)
	err := n.insert([]byte("kacky-24"), []byte("mehul"))
	if err != nil {
		t.Fatalf("got an error on insertion: %v", err)
	}

	t.Log(debugPrint(n, 100))

	k, v := n.getKV(0)
	if string(k) != "kacky" || string(v) != "mehul" {
		t.Fatalf("first kv mismatch")
	}
}

func TestNodeLeafNodeInsert2(t *testing.T) {
	buf := make([]byte, 4096)
	n := NewNode(buf)
	err := n.insert([]byte("kacky"), []byte("mehul"))
	if err != nil {
		t.Fatalf("got an error on insertion: %v", err)
	}

	t.Log(debugPrint(n, 100))

	err = n.insert([]byte("kacky11"), []byte("mehul11"))
	if err != nil {
		t.Fatalf("got an error on second insertion: %v", err)
	}

	t.Log(debugPrint(n, 100))

	k, v := n.getKV(0)
	if string(k) != "kacky" || string(v) != "mehul" {
		t.Fatalf("first kv mismatch")
	}
	k, v = n.getKV(1)
	if string(k) != "kacky11" || string(v) != "mehul11" {
		t.Fatalf("second kv mismatch")
	}
}

func TestNodeLeafNodeInsert3(t *testing.T) {
	buf := make([]byte, 4096)
	n := NewNode(buf)
	for i := range 175 {
		k := fmt.Sprintf("kacky-%d", i)
		err := n.insert([]byte(k), []byte("mehul"))
		if err != nil {
			t.Fatalf("got an error on insertion: %v", err)
			break
		}
	}

	t.Logf("node is filled to %v bytes\n", n.getSize())

	err := n.insert([]byte("a"), []byte("b"))
	if err == nil {
		t.Fatalf("should've thrown an overflow error: %v", err)
	}
}
