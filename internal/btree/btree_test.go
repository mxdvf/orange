package btree

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func init() {
	err := os.MkdirAll("test/", 0755)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setup(t *testing.T, sync bool) (*BTree, string) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	if t != nil {
		t.Logf("running test case for file: %v", filename)
	}
	tree, err := NewBTree(filename, sync)
	if err != nil {
		t.Fatalf("cannot initialize tree: %v", err)
	}

	return tree, filename
}

func TestBtreeInitialize(t *testing.T) {
	tree, _ := setup(t, true)

	r, err := tree.pm.Read(tree.root)
	if err != nil {
		t.Fatal(err.Error())
	}

	if NewNode(r).getType() != NodeTypeLeaf {
		t.Fatal("root should've been a leaf page the very first time")
	}
}

func TestBtreeSimpleInsert1(t *testing.T) {
	tree, _ := setup(t, true)

	k := []byte("ducky")
	v := []byte("mehul")
	if err := tree.Insert(k, v); err != nil {
		fmt.Println("wut1")
		t.Fatalf("insert failed: %v", err)
	}

	buf, _ := tree.pm.Read(tree.root)
	node := NewNode(buf)

	if node.getNKeys() != 1 {
		t.Fatal("node should have only 1 key")
	}

	k1, v1 := node.getKV(0)
	if res := bytes.Compare(k, k1); res != 0 {
		t.Fatal("keys don't match up")
	}
	if res := bytes.Compare(v, v1); res != 0 {
		t.Fatal("vals don't match up")
	}
}

func TestBtreeSimpleInsert2(t *testing.T) {
	tree, _ := setup(t, true)

	k := []byte("ducky")
	v := []byte("mehul")
	if err := tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	buf, _ := tree.pm.Read(tree.root)
	node := NewNode(buf)
	if node.getNKeys() != 1 {
		t.Fatal("node should have only 1 key")
	}
	k1, v1 := node.getKV(0)
	if res := bytes.Compare(k, k1); res != 0 {
		t.Fatal("keys don't match up")
	}
	if res := bytes.Compare(v, v1); res != 0 {
		t.Fatal("vals don't match up")
	}

	k = []byte("ducky11")
	v = []byte("mehul11")
	if err := tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	buf, _ = tree.pm.Read(tree.root)
	node = NewNode(buf)
	if node.getNKeys() != 2 {
		t.Fatal("node should have 2 keys")
	}
	k1, v1 = node.getKV(1)
	if res := bytes.Compare(k, k1); res != 0 {
		t.Fatal("keys don't match up")
	}
	if res := bytes.Compare(v, v1); res != 0 {
		t.Fatal("vals don't match up")
	}
}

func TestBtreeFillUntilRootSplits1Level(t *testing.T) {
	tree, _ := setup(t, true)

	kNums := []string{"10", "15", "20", "21", "16", "12", "2"}

	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		if err := tree.Insert([]byte(k), []byte("mehul")); err != nil {
			t.Fatalf("got an error on insertion: %v", err)
		}
	}

	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		v, err := tree.Search([]byte(k))
		if err != nil {
			t.Fatal(err)
		}
		if v == nil {
			t.Fatalf("value exists because the key has been inserted: %v", k)
		}
	}
}

func TestBtreeFillUntilRootSplits2Level(t *testing.T) {
	tree, _ := setup(t, true)

	kNums := []string{"10", "17", "20", "21", "16", "12", "2", "1", "3", "4", "11"}
	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		if err := tree.Insert([]byte(k), []byte("mehul")); err != nil {
			t.Fatalf("got an error on insertion: %v", err)
		}
	}

	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		v, err := tree.Search([]byte(k))
		if err != nil {
			t.Fatal(err)
		}
		if v == nil {
			t.Fatalf("value exists because the key has been inserted: %v", k)
		}
	}
}

func TestBtreeUnboundedInsert(t *testing.T) {
	tree, _ := setup(t, false)

	for i := range 10000 {
		k := strings.Repeat("A", 1338-len(strconv.Itoa(i))) + "_" + strconv.Itoa(i)
		if err := tree.Insert([]byte(k), []byte("mehul")); err != nil {
			t.Fatalf("got an error on insertion: %v", err)
		}
	}

	for i := range 10000 {
		k := strings.Repeat("A", 1338-len(strconv.Itoa(i))) + "_" + strconv.Itoa(i)
		v, err := tree.Search([]byte(k))
		if err != nil {
			t.Fatalf("search error: %v", err)
		}
		if v == nil {
			t.Fatalf("value exists because the key has been inserted: %v", k)
		}
	}
}

func TestBtreeSimpleDelete1(t *testing.T) {
	tree, _ := setup(t, true)

	k, v := []byte("ducky"), []byte("mehul")
	if err := tree.Insert(k, v); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if err := tree.Delete(k); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// tree should be empty — search must fail
	val, err := tree.Search(k)
	if err == nil || val != nil {
		t.Fatal("key should not exist after deletion")
	}
}

func TestBtreeSimpleDelete2(t *testing.T) {
	tree, _ := setup(t, true)

	keys := [][]byte{[]byte("apple"), []byte("banana"), []byte("cherry")}
	for _, k := range keys {
		if err := tree.Insert(k, []byte("mehul")); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	// delete the middle key
	if err := tree.Delete([]byte("banana")); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// deleted key must not exist
	val, err := tree.Search([]byte("banana"))
	if err == nil || val != nil {
		t.Fatal("deleted key should not exist")
	}

	// remaining keys must still exist
	for _, k := range [][]byte{[]byte("apple"), []byte("cherry")} {
		v, err := tree.Search(k)
		if err != nil || v == nil {
			t.Fatalf("key %s should still exist after unrelated deletion", k)
		}
	}
}

func TestBtreeDeleteFirstKey(t *testing.T) {
	tree, _ := setup(t, true)

	kNums := []string{"10", "15", "20", "21", "16", "12", "2"}
	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		tree.Insert([]byte(k), []byte("mehul"))
	}

	// delete the lexicographically smallest key
	smallest := strings.Repeat("A", 1338-len("10")) + "_10"
	if err := tree.Delete([]byte(smallest)); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	val, err := tree.Search([]byte(smallest))
	if err == nil || val != nil {
		t.Fatal("deleted key should not exist")
	}
}

func TestBtreeDeleteLastKey(t *testing.T) {
	tree, _ := setup(t, true)

	kNums := []string{"10", "15", "20", "21", "16", "12", "2"}
	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		tree.Insert([]byte(k), []byte("mehul"))
	}

	// delete the lexicographically largest key
	largest := strings.Repeat("A", 1338-len("2")) + "_2"
	if err := tree.Delete([]byte(largest)); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	val, err := tree.Search([]byte(largest))
	if err == nil || val != nil {
		t.Fatal("deleted key should not exist")
	}
}

func TestBtreeDeleteNonExistentKey(t *testing.T) {
	tree, _ := setup(t, true)

	tree.Insert([]byte("ducky"), []byte("mehul"))

	err := tree.Delete([]byte("nonexistent"))
	if err == nil {
		t.Fatal("expected error when deleting nonexistent key")
	}
}

func TestBtreeDeleteCausesUnderflowAndRotatesRight(t *testing.T) {
	// force a right rotation: left sibling must have enough keys to donate
	tree, _ := setup(t, true)

	kNums := []string{"10", "15", "20", "21", "16", "12", "2"}
	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		tree.Insert([]byte(k), []byte("mehul"))
	}

	// tree.print()

	// delete until a rotation is forced
	toDelete := []string{"20", "21", "16"}
	for _, kNum := range toDelete {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		if err := tree.Delete([]byte(k)); err != nil {
			t.Fatalf("delete failed: %v", err)
		}
	}

	// tree.print()

	// verify remaining keys
	remaining := []string{"10", "15", "12", "2"}
	for _, kNum := range remaining {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		v, err := tree.Search([]byte(k))
		if err != nil || v == nil {
			t.Fatalf("key %s should still exist", kNum)
		}
	}
}

func TestBtreeDeleteCausesUnderflowAndRotatesLeft(t *testing.T) {
	// force a right rotation: left sibling must have enough keys to donate
	tree, _ := setup(t, true)

	kNums := []string{"10", "15", "20", "21", "16", "12", "2"}
	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		tree.Insert([]byte(k), []byte("mehul"))
	}

	// delete until a rotation is forced
	toDelete := []string{"12", "2", "10"}
	for _, kNum := range toDelete {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		if err := tree.Delete([]byte(k)); err != nil {
			t.Fatalf("delete failed: %v", err)
		}
	}

	// verify remaining keys
	remaining := []string{"15", "16", "20", "21"}
	for _, kNum := range remaining {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		v, err := tree.Search([]byte(k))
		if err != nil || v == nil {
			t.Fatalf("key %s should still exist", kNum)
		}
	}
}

func TestBtreeDeleteCausesMerge(t *testing.T) {
	tree, _ := setup(t, true)

	kNums := []string{"10", "17", "20", "21", "16", "12", "2", "1", "3", "4", "11"}
	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		tree.Insert([]byte(k), []byte("mehul"))
	}

	// delete enough keys to force a merge
	toDelete := []string{"20", "21", "17", "16"}
	for _, kNum := range toDelete {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		if err := tree.Delete([]byte(k)); err != nil {
			t.Fatalf("delete failed for key %s: %v", kNum, err)
		}
	}

	// verify deleted keys are gone
	for _, kNum := range toDelete {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		val, err := tree.Search([]byte(k))
		if err == nil || val != nil {
			t.Fatalf("key %s should not exist after deletion", kNum)
		}
	}

	// verify remaining keys intact
	remaining := []string{"10", "12", "2", "1", "3", "4", "11"}
	for _, kNum := range remaining {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		v, err := tree.Search([]byte(k))
		if err != nil || v == nil {
			t.Fatalf("key %s should still exist", kNum)
		}
	}
}

func TestBtreeDeleteFromInternalNodeTwice(t *testing.T) {
	tree, _ := setup(t, true)

	kNums := []string{"10", "17", "20", "21", "16", "12", "2", "1", "3", "4", "11"}
	for _, kNum := range kNums {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		tree.Insert([]byte(k), []byte("mehul"))
	}

	// delete a key that is likely sitting in an internal node as a separator
	toDelete := []string{"17", "12"}
	for _, kNum := range toDelete {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		if err := tree.Delete([]byte(k)); err != nil {
			t.Fatalf("delete failed: %v", err)
		}
	}

	for _, kNum := range toDelete {
		val, err := tree.Search([]byte(kNum))
		if err == nil || val != nil {
			t.Fatal("deleted key should not exist")
		}
	}

	// all other keys must remain
	remaining := []string{"10", "20", "21", "16", "2", "1", "3", "4", "11"}
	for _, kNum := range remaining {
		k := strings.Repeat("A", 1338-len(kNum)) + "_" + kNum
		v, err := tree.Search([]byte(k))
		if err != nil || v == nil {
			t.Fatalf("key %s should still exist", kNum)
		}
	}
}

func TestBtreeUnboundedDelete(t *testing.T) {
	tree, _ := setup(t, false)
	n := 2000

	for i := range n {
		k := strings.Repeat("A", 1338-len(strconv.Itoa(i))) + "_" + strconv.Itoa(i)
		if err := tree.Insert([]byte(k), []byte("mehul")); err != nil {
			t.Fatalf("insert failed at i=%d: %v", i, err)
		}
	}

	// delete all keys in reverse order
	for i := n - 1; i >= 0; i-- {
		k := strings.Repeat("A", 1338-len(strconv.Itoa(i))) + "_" + strconv.Itoa(i)
		if err := tree.Delete([]byte(k)); err != nil {
			t.Fatalf("delete failed at i=%d: %v", i, err)
		}
	}

	// none should exist
	for i := range n {
		k := strings.Repeat("A", 1338-len(strconv.Itoa(i))) + "_" + strconv.Itoa(i)
		val, err := tree.Search([]byte(k))
		if err == nil || val != nil {
			t.Fatalf("key %d should not exist after deletion", i)
		}
	}
}

func TestBtreeInterleaveInsertDelete(t *testing.T) {
	tree, _ := setup(t, true)
	inserted := map[string]bool{}

	for i := range 500 {
		k := strings.Repeat("A", 1338-len(strconv.Itoa(i))) + "_" + strconv.Itoa(i)
		if err := tree.Insert([]byte(k), []byte("mehul")); err != nil {
			t.Fatalf("insert failed at i=%d: %v", i, err)
		}
		inserted[k] = true

		// every 10 inserts, delete the key from 5 steps ago
		if i >= 5 && i%10 == 0 {
			j := i - 5
			delKey := strings.Repeat("A", 1338-len(strconv.Itoa(j))) + "_" + strconv.Itoa(j)
			if err := tree.Delete([]byte(delKey)); err != nil {
				t.Fatalf("delete failed at i=%d j=%d: %v", i, j, err)
			}
			inserted[delKey] = false
		}
	}

	// verify final state
	for k, exists := range inserted {
		val, err := tree.Search([]byte(k))
		if exists && (err != nil || val == nil) {
			t.Fatalf("key %s should exist", k)
		}
		if !exists && (err == nil || val != nil) {
			t.Fatalf("key %s should not exist", k)
		}
	}
}

// benchmarks

func BenchmarkInsert(b *testing.B) {
	tr, _ := setup(nil, true)
	val := []byte("mehul")
	var i uint64

	b.ResetTimer()
	for b.Loop() {
		// Fast string concatenation + a single slice allocation
		k := []byte("kacky-" + strconv.FormatUint(i, 10))
		i++

		if err := tr.Insert(k, val); err != nil && err != ErrOverflow {
			b.Fatalf("insertion failed: %v", err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	// pre-populate the tree with 100k keys before benchmarking with fsync switched off
	tr, _ := setup(nil, false)
	val := []byte("mehul")
	const numKeys = 100_000
	for i := range uint64(numKeys) {
		k := []byte("kacky-" + strconv.FormatUint(i, 10))
		if err := tr.Insert(k, val); err != nil && err != ErrOverflow {
			b.Fatalf("setup insertion failed: %v", err)
		}
	}

	time.Sleep(2 * time.Second)

	var i uint64
	// reset timer so setup cost is excluded from benchmark
	b.ResetTimer()
	for b.Loop() {
		// scatter across the full key space — not sequential
		// this forces the btree to traverse different paths each time
		idx := (i * 6364136223846793005) % numKeys
		k := []byte("kacky-" + strconv.FormatUint(idx, 10))
		i++
		if _, err := tr.Search(k); err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}
