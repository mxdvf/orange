package btree

import (
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

	if NewNode(r).getType() != NODE_TYPE_LEAF {
		t.Fatal("root should've been a leaf page the very first time")
	}
}

func TestBtreeSimpleInsert1(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	k := []byte("kacky")
	v := []byte("mehul")
	if err = tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	t.Log(tree.root)
	buf, _ := tree.pm.read(tree.root)
	t.Log(buf[:100])
}

func TestBtreeSimpleInsert2(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	buf, err := tree.pm.read(tree.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("looking at root(%v) before inserting: %v", tree.root, buf[:100])

	k := []byte("kacky")
	v := []byte("mehul")
	if err = tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	buf, err = tree.pm.read(tree.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("looking at root(%v) after inserting one pair: %v", tree.root, buf[:100])

	k = []byte("kacky11")
	v = []byte("mehul11")
	if err = tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	buf, err = tree.pm.read(tree.root)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("looking at root(%v) after inserting two pairs: %v", tree.root, buf[:100])
}

func TestBtreeNonSplitMultipleKeys(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	for i := range 174 {
		k := fmt.Sprintf("kacky-%d", i)
		err := tree.Insert([]byte(k), []byte("mehul"))
		if err != nil {
			t.Fatalf("got an error on insertion: %v", err)
			break
		}
	}
}

func TestBtreeSplitRoot(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	var keyCount uint16 = 0

	for i := range 174 {
		k := fmt.Sprintf("kacky-%d", i)
		err := tree.Insert([]byte(k), []byte("mehul"))
		if err != nil {
			t.Fatalf("got an error on insertion: %v", err)
		}
		keyCount++
	}

	k1, v1 := []byte("kacky-175"), []byte("mehul")
	tree.Insert(k1, v1)
	keyCount++
	// ---- the root split has happened by now ---- //

	// reading child 1
	buf, _ := tree.pm.read(179)
	left := NewNode(buf)

	// reading child 2
	buf, _ = tree.pm.read(176)
	right := NewNode(buf)

	// naive check
	if left.getNKeys()+right.getNKeys()+1 != keyCount {
		t.Fatal("nope something went seriously wrong in between, some keys got lost")
	}
}

func TestBtreeSplitInternalNode(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	for i := range 174 {
		k := fmt.Sprintf("kacky-%d", i)
		err := tree.Insert([]byte(k), []byte("mehul"))
		if err != nil {
			t.Fatalf("got an error on insertion: %v", err)
		}
	}

	k, v := []byte("kacky-175"), []byte("mehul")
	tree.Insert(k, v)
	// ---- root split happened here ---- //

	buf, _ := tree.pm.read(tree.root)
	root := NewNode(buf)
	if root.getNKeys() != 1 {
		t.Fatal("major bug, print `root.data`")
	}
	// fmt.Println(root.data)

	// reading child 1
	buf, _ = tree.pm.read(root.getPtr(0))
	left := NewNode(buf)
	// reading child 2
	buf, _ = tree.pm.read(root.getPtr(1))
	right := NewNode(buf)
	if left.getNKeys() != 87 || right.getNKeys() != 87 {
		t.Fatal("expected both left and right child to have 87 keys")
	}

	// ---- at this point, root should have 1 key, left and right child each should have 87 keys = totalling upto 175 keys ---- //

	for i := range 91 {
		k := fmt.Sprintf("backy-%d", i)
		err := tree.Insert([]byte(k), []byte("mehul"))
		if err != nil {
			t.Fatalf("got an error on insertion: %v", err)
		}
	}

	buf, _ = tree.pm.read(tree.root)
	root = NewNode(buf)

	// reading child 1
	buf, _ = tree.pm.read(root.getPtr(0))
	left = NewNode(buf)
	// reading child 2
	buf, _ = tree.pm.read(root.getPtr(1))
	right = NewNode(buf)
	if left.getNKeys() != 178 || right.getNKeys() != 87 {
		t.Fatal("expected left and right child to have 178 and 87 keys respectively")
	}

	// ---- now we add 91 keys to the left child specifically which makes left child's keys = 178 keys ONLY IN THE LEFT CHILD ---- //
	// ---- root should still have 1 key and right child should still possess 87 keys ---- //

	k, v = []byte("a"), []byte("mehul")
	err = tree.Insert(k, v)
	if err != nil {
		t.Fatalf("something went wrong during insertion: %v", err)
	}

	// root
	buf, _ = tree.pm.read(tree.root)
	root = NewNode(buf)
	if root.getNKeys() != 2 {
		t.Fatal("major bug, root should have 2 keys by now")
	}

	// reading left child
	buf, _ = tree.pm.read(root.getPtr(0))
	left = NewNode(buf)
	// reading middle child
	buf, _ = tree.pm.read(root.getPtr(1))
	middle := NewNode(buf)
	// reading right child
	buf, _ = tree.pm.read(root.getPtr(2))
	right = NewNode(buf)
	// check
	if left.getNKeys() != 89 || middle.getNKeys() != 89 || right.getNKeys() != 87 {
		t.Fatal("major bug, child(0), child(1), child(2) should have 89, 89, 87 keys respectively")
	}

	// ---- now we added 1 more key to the left child which made it overflow ---- //
	// ---- root should now have 2 keys; child(0) = 89 keys, child(1) = 89 keys, child(2) = 87 keys ---- //
	// ---- the addition works, 178 keys broken down into 1/2 with 1 key handed over to the parent ---- //

	k, v = []byte("dacky-1"), []byte("mehul")
	err = tree.Insert(k, v)
	if err != nil {
		t.Fatalf("failed to insert the key: %v", err)
	}

	// root
	buf, _ = tree.pm.read(tree.root)
	root = NewNode(buf) // need this here to re-initialize root

	// reading left child
	buf, _ = tree.pm.read(root.getPtr(0))
	left = NewNode(buf)
	// reading middle child
	buf, _ = tree.pm.read(root.getPtr(1))
	middle = NewNode(buf)
	// reading right child
	buf, _ = tree.pm.read(root.getPtr(2))
	right = NewNode(buf)
	// check
	if left.getNKeys() != 89 || middle.getNKeys() != 90 || right.getNKeys() != 87 {
		t.Fatal("major bug, child(0), child(1), child(2) should have 89, 90, 87 keys respectively")
	}

	// // ---- now we added 1 more key to the middle child of root aka child(1) ---- //
	// // ---- simply put, child(1) should have 90 keys now ---- //
}

func TestBtreeSearch(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	k := []byte("kacky1")
	v := []byte("mehul2")
	if err = tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	v, err = tree.Search(k)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	t.Logf("key: %v has value: %v", string(k), string(v))
}

func TestBtreeSearchOnceAfterMultipleKeys(t *testing.T) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	t.Logf("running test case for file: %v", filename)
	tree, err := NewBTree(filename)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	for i := range 300 {
		k := fmt.Sprintf("kacky-%d", i)
		err := tree.Insert([]byte(k), []byte("mehul"))
		if err != nil {
			t.Fatalf("got an error on insertion: %v", err)
		}
	}
	// TODO: even though my logic is page number agnostic, it fails at anything near 400/500 -- i am assuming this maybe because the file gets too large

	t.Logf("let's look at the page number of root: %v", tree.root)

	k1 := "kacky-145"
	v, err := tree.Search([]byte(k1))
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	t.Logf("key: %v has value: %v", string(k1), string(v))
}
