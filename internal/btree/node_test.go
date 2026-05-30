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

	k, v := []byte("ducky-24"), []byte("mehul")
	n.insertKV(k, v)

	k1, v1 := n.getKV(0)
	if string(k) != string(k1) || string(v) != string(v1) {
		t.Fatalf("first kv mismatch")
	}
}

func TestNodeLeafNodeInsert2(t *testing.T) {
	buf := make([]byte, 4096)
	n := NewNode(buf)
	n.insertKV([]byte("ducky"), []byte("mehul"))

	n.insertKV([]byte("ducky11"), []byte("mehul11"))

	k, v := n.getKV(0)
	if string(k) != "ducky" || string(v) != "mehul" {
		t.Fatalf("first kv mismatch")
	}
	k, v = n.getKV(1)
	if string(k) != "ducky11" || string(v) != "mehul11" {
		t.Fatalf("second kv mismatch")
	}
}

func TestNodeLeafNodeInsert3(t *testing.T) {
	buf := make([]byte, 4096)
	n := NewNode(buf)
	for i := range 174 {
		k, v := []byte(fmt.Sprintf("ducky-%d", i)), []byte("mehul")
		n.insertKV(k, v)
	}

	t.Logf("node is filled to %v/%v bytes\n", n.getSize(), PageSize)
	k1, v1 := []byte("ducky-175"), []byte("mehul")
	t.Logf("and about to insert a kv pair of post-insert size: %v\n", n.getTotalLenIfInserted(k1, v1))

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				// check it's the right panic
				if msg, ok := r.(string); ok && msg == "illegal node, it should have been split by a preemptive fix" {
					panicked = true
				} else {
					t.Errorf("wrong panic: %v", r)
				}
			}
		}()
		n.insertKV(k1, v1)
	}()

	if !panicked {
		t.Fatal("expected a panic on overflow but got none")
	}
}

func TestNodeLeafNodeInsert4VisualizePtrBehaviour(t *testing.T) {
	buf := make([]byte, 4096)
	n := NewNode(buf)

	for i := range 1 {
		k, v := []byte(fmt.Sprintf("%d", i)), []byte("ZZZZZ")
		n.insertKV(k, v)
	}

	n.setPtr(0, 11)
	n.setPtr(1, 12)

	t.Log(n.data[:50])

	k, v := []byte(fmt.Sprintf("%d", 2)), []byte("ZZZZZ")
	danglingPtr1 := n.getPtr(n.getNKeys())
	n.insertKV(k, v)
	n.setPtr(2, 13)
	danglingPtr2 := n.getPtr(n.getNKeys())
	n.setPtr(n.getNKeys()-1, danglingPtr1)
	n.setPtr(n.getNKeys(), danglingPtr2)

	t.Log(n.data[:50])
}

func TestNodeLeafNodeDelete1VisualizePtrBehaviour(t *testing.T) {
	buf := make([]byte, 4096)
	n := NewNode(buf)

	for i := range 1 {
		k, v := []byte(fmt.Sprintf("%d", i)), []byte("ZZZZZ")
		n.insertKV(k, v)
	}

	n.setPtr(0, 11)
	n.setPtr(1, 12)

	t.Log(n.data[:50])

	n.deleteKV(0)

	t.Log(n.data[:50])
}
