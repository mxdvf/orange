package btree

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"testing"
)

func init() {
	err := os.MkdirAll("test/", 0755)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func TestBtreeInitialize(t *testing.T) {
	tree, err := NewBTree("test/test.bin")
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	r, err := tree.pm.read(tree.root)
	if err != nil {
		t.Fatal(err.Error())
	}

	ntype := binary.BigEndian.Uint16(r[0:])
	if ntype != NODE_TYPE_LEAF {
		t.Fatal("root should've been a leaf page the very first time")
	}
}

func TestBtreeSimpleInsert(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	buf, err := tree.pm.read(1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("looking at root(1) before inserting: %v", buf[:100])

	k := []byte("kacky")
	v := []byte("mehul")
	if err = tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	buf, err = tree.pm.read(2)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("looking at root(2) after inserting one pair: %v", buf[:100])

	k = []byte("kacky11")
	v = []byte("mehul11")
	if err = tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	buf, err = tree.pm.read(3)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("looking at root(3) after inserting two pairs: %v", buf[:100])
}

func TestBtreeNonSplitMultipleKeys(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	for i := range 175 {
		k := fmt.Sprintf("kacky-%d", i)
		err := tree.Insert([]byte(k), []byte("----"))
		if err != nil {
			t.Fatalf("got an error on insertion: %v", err)
			break
		}
	}
}
