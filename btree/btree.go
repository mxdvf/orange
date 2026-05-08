package btree

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	NODE_TYPE_LEAF int = iota
	NODE_TYPE_INTERNAL
)

const (
	HEADER_SIZE  = 4 // 2 + 2 = 4 bytes
	PTR_SIZE     = 4 // 4 bytes
	OFFSET_SIZE  = 2 // 2 bytes
	KEY_LEN_SIZE = 2 // 2 bytes
	VAL_LEN_SIZE = 2 // 2 bytes
)

type Node struct {
	// wire format:
	// type  |  nkeys |   pointers  |  offset-array	 |		     key-values
	//  2B   |   2B   |  nkeys * 4B |  	nkeys * 2B	 |  [klen: 2B][k][vlen: 2B][v]
	data []byte
}

// TODO:
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
	fmt.Println("only showing top 100 bytes:")
	fmt.Println(n.data[:100])
}

func (n *Node) getType() int {
	return int(binary.BigEndian.Uint16(n.data[0:]))
}

func (n *Node) getNKeys() int {
	return int(binary.BigEndian.Uint16(n.data[2:]))
}

func (n *Node) getAppropriateIdx(target []byte) (int, int) {
	// TODO: switch to binary search, right now
	// this is standard linear search
	start := HEADER_SIZE + n.getNKeys()*PTR_SIZE
	var offsetPos, nextOffsetPos uint16
	for i := 0; i < n.getNKeys(); i++ {
		offsetPos = binary.BigEndian.Uint16(n.data[start+(i*OFFSET_SIZE):])

		key := n.getKeyByOffset(offsetPos)
		if res := bytes.Compare(target, key); res == -1 || res == 0 {
			nextOffsetPos = uint16(start + ((i + 1) * OFFSET_SIZE))
			break
		}
	}
	return int(offsetPos), int(nextOffsetPos)
}

func (n *Node) getKeyByOffset(offset uint16) []byte {
	keyLen := binary.BigEndian.Uint16(n.data[offset:])

	start := offset + KEY_LEN_SIZE
	end := offset + KEY_LEN_SIZE + keyLen

	return n.data[start : end+1]
}

// TODO: we're assuming the key will fit into the node
// TODO: we're assuming the key to be inserted is unique
// TODO: we're assuming the key is not large enough to destroy a node (let's say node is empty and it occupies all the space)
func (n *Node) insertKvPair(k, v []byte) {
}
