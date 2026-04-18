package main

import (
	"fmt"
	"slices"
)

const BRANCHING_FACTOR_T = 2

const MAX_CHILDREN_PER_NODE = 2 * BRANCHING_FACTOR_T
const MAX_KEYS_PER_NODE = (2 * BRANCHING_FACTOR_T) - 1

type Node struct {
	keys     []uint16
	children []*Node
	isLeaf   bool
}

func NewNode(isLeaf bool) *Node {
	return &Node{
		keys:     make([]uint16, 0, MAX_KEYS_PER_NODE),
		children: make([]*Node, MAX_CHILDREN_PER_NODE),
		isLeaf:   isLeaf,
	}
}

func (n *Node) insertInSelf(k uint16) {
	n.keys = append(n.keys, k)
	slices.Sort(n.keys)
}

type BTree struct {
	root *Node
	t    int
}

func (t *BTree) Insert(n *Node, k uint16) {
	// Case A: if the node is not a leaf, then k goes inside one of its child
	if !t.root.isLeaf {
		var (
			idx   int
			found bool
		)
		for idx = range t.root.keys {
			if k < t.root.keys[idx] {
				t.Insert(t.root.children[idx], k)
				found = true
				return
			}
		}
		if !found {
			t.Insert(t.root.children[idx+1], k)
			return
		}
	}

	// Case B: if the node is a leaf and has space, then k goes inside of its self
	if len(t.root.keys) < MAX_KEYS_PER_NODE {
		t.root.insertInSelf(k)
		return
	}

	// Case C: if the node is a leaf but does not have space, perform split
	root, n1, n2 := NewNode(false), NewNode(true), NewNode(true)
}

func main() {
	tree := &BTree{
		root: NewNode(true),
		t:    BRANCHING_FACTOR_T,
	}

	tree.Insert(tree.root, 10)
	tree.Insert(tree.root, 11)
	tree.Insert(tree.root, 20)
	tree.Insert(tree.root, 21)
	tree.Insert(tree.root, 4)
	fmt.Println(tree.root.keys)
	fmt.Println(tree.root.children)
}
