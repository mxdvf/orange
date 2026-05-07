package btree

import (
	"encoding/binary"
	"fmt"
)

const (
	NODE_TYPE_LEAF int = iota
	NODE_TYPE_INTERNAL
)

const (
	HEADER_SIZE = 4  // 4 bytes
	PTR_SIZE    = 2  // 2 bytes
	KEY_SIZE    = 8  // 8 bytes
	VALUE_SIZE  = 16 // 16 bytes
)

type Node struct {
	data []byte
	//  type  |  nkeys |   pointers  |  key-values
	//   2B   |   2B   |  nkeys * 2B |  nkeys * 24B
}

// TODO:
// 0. fuck the book's design -- it's genuinely rubbish
// 1. keep this node in memory but interact with bytes -- do not think of persistence yet
// 3. come up with code that can ONLY READ FOR NOW the pointers and key-value pairs
// 4. forget everything and only perform operations assume only this node exists, remove/insert keys, etc.

func NewLeafNode(nkeys uint16) *Node {
	return newNode(NODE_TYPE_LEAF, nkeys)
}

func NewInternalNode(nkeys uint16) *Node {
	return newNode(NODE_TYPE_INTERNAL, nkeys)
}

func newNode(t int, nkeys uint16) *Node {
	n := &Node{
		data: make([]byte, 4096),
	}
	binary.BigEndian.PutUint16(n.data[0:], uint16(t))
	binary.BigEndian.PutUint16(n.data[2:], nkeys)
	return n
}

// Only for internal debugging
func (n *Node) debugPrint() {
	fmt.Println("only showing top 10 bytes:")
	fmt.Println(n.data[:10])
}

func (n *Node) nodeType() uint16 {
	return binary.BigEndian.Uint16(n.data[0:])
}

func (n *Node) nkeys() uint16 {
	return binary.BigEndian.Uint16(n.data[2:])
}

func (n *Node) getPtr(idx int) uint16 {
	offset := HEADER_SIZE + idx*PTR_SIZE
	return binary.BigEndian.Uint16(n.data[offset:])
}

func (n *Node) setPtr(idx int, v uint16) {
	offset := HEADER_SIZE + idx*PTR_SIZE
	binary.BigEndian.PutUint16(n.data[offset:], v)
}

// TODO: linear search for the keys
