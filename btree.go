package main

import (
	"fmt"
	"slices"
	"strings"
)

const BRANCHING_FACTOR_T = 2

const MAX_KEYS_PER_NODE = (2 * BRANCHING_FACTOR_T) - 1
const MAX_CHILDREN_PER_NODE = 2 * BRANCHING_FACTOR_T

type Node struct {
	keys     []uint16
	children []*Node
	isLeaf   bool
}

func NewNode(isLeaf bool, keys []uint16) *Node {
	node := &Node{
		keys:     make([]uint16, 0, MAX_KEYS_PER_NODE),
		children: make([]*Node, MAX_CHILDREN_PER_NODE),
		isLeaf:   isLeaf,
	}
	if keys != nil {
		node.keys = keys
	}
	return node
}

type BTree struct {
	root *Node
}

func (t *BTree) Insert(k uint16) {
	if len(t.root.keys) == MAX_KEYS_PER_NODE {
		// break the root down before going any further
		newRoot := NewNode(false, nil)
		t.splitChild(newRoot, t.root, k)
		t.root = newRoot

		// perform the insert operation as intended
		t.insertInSubtree(t.root, k)
	} else {
		fmt.Println("48 ki fielding bithaayi hai", t.root.keys, k)
		// perform the insert operation as intended
		t.insertInSubtree(t.root, k)
	}
}

func (t *BTree) insertInSubtree(node *Node, k uint16) {
	fmt.Println("fielding dekhte hai kahan pohochi", node.keys, node.isLeaf, k)
	// Case A: node is a leaf and has space
	if node.isLeaf && len(node.keys) < MAX_KEYS_PER_NODE {
		t.insertInNode(node, k)
		return
	}

	// Case B: node is not a leaf
	if !node.isLeaf {
		idx := t.calculateAppropriateIdx(node.keys, k)
		fmt.Println("deep internals tak pohoch chuka hoon", node.children, idx, k)
		switch {
		// Case B1: if child is not a leaf, keep traversing
		case !node.children[idx].isLeaf:
			t.insertInSubtree(node.children[idx], k)

		// Case B2: if appropriate child (a leaf) has space, insert there
		case node.children[idx].isLeaf && len(node.children[idx].keys) < MAX_KEYS_PER_NODE:
			t.insertInNode(node.children[idx], k)

			// Case B3: if appropriate child (a leaf) does not have space, perform split
		case node.children[idx].isLeaf && len(node.children[idx].keys) == MAX_KEYS_PER_NODE:
			t.splitChild(node, node.children[idx], k)
		}
	}
}

func (t *BTree) insertInNode(node *Node, k uint16) {
	// calculates the appropriate index and inserts `k`
	idx := t.calculateAppropriateIdx(node.keys, k)
	node.keys = slices.Insert(node.keys, idx, k)
}

func (t *BTree) splitChild(parent *Node, child *Node, k uint16) {
	// order keys to prepare them for a split
	tempKeys := make([]uint16, MAX_KEYS_PER_NODE)
	copy(tempKeys, child.keys)
	if child != t.root {
		idx := t.calculateAppropriateIdx(tempKeys, k)
		tempKeys = slices.Insert(tempKeys, idx, k)
	}

	median := len(tempKeys) / 2

	// parent stores the derived median key
	idx := t.calculateAppropriateIdx(parent.keys, tempKeys[median])
	parent.keys = slices.Insert(parent.keys, idx, tempKeys[median])

	// setup new child nodes
	left, right := NewNode(true, tempKeys[:median]), NewNode(true, tempKeys[median+1:])
	fmt.Printf("konsa baap ki aulaad yahan se jaane wala hai: %+v and %d\n", parent.children, idx)
	parent.children[idx] = nil // TODO: shady line, the logic here is that instead of creating two nodes, create just one, and manipulate the other existing one
	parent.children = slices.Insert(parent.children, idx, left)
	parent.children = slices.Insert(parent.children, idx+1, right)

	if child == t.root && !child.isLeaf {
		left.children = child.children[:median+1]
		right.children = child.children[median+1:]
		// TODO: change to copy function if too much issues

		left.isLeaf = false
		right.isLeaf = false
	}
	fmt.Printf("konsa baap ki aulaad yahan se jaane wala hai: %+v and %d\n", parent.children, idx)
}

// calculateAppropriateIdx returns an int which provides the appropriate
// index within the "sorted" slice of keys. Used for finding appropriate
// child or appropriate position for a key
func (t *BTree) calculateAppropriateIdx(nodeKeys []uint16, k uint16) int {
	var idx int
	for idx = 0; idx < len(nodeKeys); idx++ {
		if k < nodeKeys[idx] {
			break
		}
	}
	return idx
}

func main() {
	tree := &BTree{
		root: NewNode(true, nil),
	}

	tree.Insert(20)
	tree.Insert(10)
	// tree.Insert(11)
	// tree.Insert(24)
	// tree.Insert(6)
	// tree.Insert(28)
	// tree.Insert(32)
	// tree.Insert(25)
	// tree.Insert(26)
	// tree.Insert(18)
	// tree.Insert(48)
	// tree.Insert(60)
	// tree.Insert(83)
	// tree.Insert(90)
	// tree.Insert(4)
	// tree.Insert(tree.root, 6)
	// tree.Insert(tree.root, 1)
	// tree.Insert(tree.root, 3)
	// tree.Insert(tree.root, 4)
	// tree.Insert(tree.root, 12)
	// tree.Insert(tree.root, 13)
	// tree.Insert(tree.root, 14)
	print(tree)
}

func print(t *BTree) {
	nodeList := [][]*Node{{t.root}}
	level := 0
	for len(nodeList[0]) != 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Level %d:\n", level))

		nodes := nodeList[0]
		for _, node := range nodes {
			if node == nil {
				continue
			}

			sb.WriteString("[")
			for i := 0; i < len(node.keys); i++ {
				sb.WriteString(fmt.Sprintf(" %v ", node.keys[i]))
			}
			sb.WriteString("]")
		}
		fmt.Println(sb.String())
		fmt.Println()

		list := []*Node{}
		for _, node := range nodes {
			if node != nil {
				list = append(list, node.children...)
			}
		}
		nodeList = append(nodeList, list)

		level++
		nodeList = nodeList[1:]
	}
}
