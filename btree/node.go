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
	// fire format:
	// type  |  nkeys |   pointers  |  offset-array	 |		     key-values
	//  2B   |   2B   |  nkeys * 4B |  	nkeys * 2B	 |  [klen: 2B][k][vlen: 2B][v]
	data []byte
}

// assumptions:
// 1. duplicate keys are not allowed
// 2. offset list contains relative positioning: check kvPos()
// 3. offset list points to the start of the next KV pair (a logically empty space to start from)

func NewLeafNode() *Node {
	return newNode(NODE_TYPE_LEAF)
}

func NewInternalNode() *Node {
	return newNode(NODE_TYPE_INTERNAL)
}

func newNode(t int) *Node {
	n := &Node{data: make([]byte, 4096)}
	binary.BigEndian.PutUint16(n.data[0:], uint16(t))
	return n
}

func debugPrint(node *Node, top int) {
	fmt.Printf("printing top %d bytes\n", top)
	fmt.Println(node.data[:top+1])
}

func (node *Node) getNKeys() uint16 {
	return binary.BigEndian.Uint16(node.data[2:])
}

func (node *Node) incrementNKeys() {
	binary.BigEndian.PutUint16(node.data[2:], node.getNKeys()+1)
}

func (node *Node) getHeaderAndMetadataLen() uint16 {
	return HEADER_SIZE + PTR_SIZE*node.getNKeys() + OFFSET_SIZE*node.getNKeys()
}

func (node *Node) offsetPos(idx uint16) uint16 {
	return HEADER_SIZE + PTR_SIZE*node.getNKeys() + OFFSET_SIZE*idx
}

func (node *Node) getOffset(idx uint16) uint16 {
	pos := node.offsetPos(idx)
	return binary.BigEndian.Uint16(node.data[pos:])
}

func (node *Node) kvPos(idx uint16) uint16 {
	return HEADER_SIZE + PTR_SIZE*node.getNKeys() + OFFSET_SIZE*node.getNKeys() + node.getOffset(idx)
}

func (node *Node) getKV(idx uint16) ([]byte, []byte) {
	pos := node.kvPos(idx)

	klen := binary.BigEndian.Uint16(node.data[pos:])
	pos += KEY_LEN_SIZE

	key := node.data[pos : pos+klen]
	pos += klen

	vlen := binary.BigEndian.Uint16(node.data[pos:])
	pos += VAL_LEN_SIZE

	val := node.data[pos : pos+vlen]

	return key, val
}

func (node *Node) putKV(k, v []byte, pos uint16) {
	binary.BigEndian.PutUint16(node.data[pos:], uint16(len(k)))
	pos += KEY_LEN_SIZE

	copy(node.data[pos:pos+uint16(len(k))], k)
	pos += uint16(len(k))

	binary.BigEndian.PutUint16(node.data[pos:], uint16(len(v)))
	pos += VAL_LEN_SIZE

	copy(node.data[pos:pos+uint16(len(v))], v)
}

func (node *Node) shiftKVRight(totalKVLen, pos uint16) {
	copy(node.data[pos+totalKVLen:], node.data[pos:])
	clear(node.data[pos : pos+totalKVLen+1])
}

func (node *Node) getKVLen(idx uint16) uint16 {
	pos := node.kvPos(idx)
	klen := binary.BigEndian.Uint16(node.data[pos:])
	vlen := binary.BigEndian.Uint16(node.data[pos+KEY_LEN_SIZE+klen:])
	return klen + vlen + KEY_LEN_SIZE + VAL_LEN_SIZE
}

func (node *Node) ptrPos(idx uint16) uint16 {
	return HEADER_SIZE + PTR_SIZE*idx
}

func (node *Node) getPtr(idx uint16) uint32 {
	pos := HEADER_SIZE + PTR_SIZE*idx
	return binary.BigEndian.Uint32(node.data[pos:])
}

func (node *Node) findInsertPos(target []byte) (uint16, uint16) {
	if node.getNKeys() <= 0 {
		return 0, node.kvPos(0)
	}

	var idx uint16
	for idx = 0; idx < node.getNKeys(); idx++ {
		k, _ := node.getKV(idx)
		if res := bytes.Compare(target, k); res == -1 || res == 0 {
			return idx, node.kvPos(idx)
		}
	}

	// for keys that would be inserted after the last kv pair
	return idx, node.kvPos(idx-1) + node.getKVLen(idx-1)
}

func (node *Node) shiftPtrAndOffsetRight(idx uint16) {
	// make space for new kv's pointer
	ptrPos := node.ptrPos(idx)
	copy(node.data[ptrPos+PTR_SIZE:], node.data[ptrPos:])
	clear(node.data[ptrPos : ptrPos+PTR_SIZE+1])

	// make space for new kv's offset
	offsetPos := node.offsetPos(idx)
	copy(node.data[offsetPos+OFFSET_SIZE:], node.data[offsetPos:])
	clear(node.data[offsetPos : offsetPos+OFFSET_SIZE+1])
}

func (node *Node) reEvaluateOffsetList(idx, calculatedPos, totalLen uint16) {
	for i := uint16(0); i < node.getNKeys(); i++ {
		pos := node.offsetPos(i)
		offsetBeforeUpdate := node.getOffset(i)
		switch {
		// anything at idx, simply insert the calculated offset.
		// there's no workaround for this beecause when inserting a key, we calculated the offset
		// for the very first time. think about this.
		case i == idx:
			binary.BigEndian.PutUint16(node.data[pos:], calculatedPos)
		// anything before idx requires no update
		// anything after idx we just need to add totalKVLen
		case i > idx:
			binary.BigEndian.PutUint16(node.data[pos:], uint16(offsetBeforeUpdate+totalLen))
		}
	}
}

func (node *Node) Insert(k, v []byte) {
	// figure out where to put the key
	insertIdx, insertPos := node.findInsertPos(k)
	node.insertInLeafNode(k, v, insertIdx, insertPos)
}

func (node *Node) insertInLeafNode(k, v []byte, insertIdx, insertPos uint16) {
	// increment nkeys (do not re-order, everything
	// after this line depends on it being here)
	node.incrementNKeys()

	// make space for pointer, offset
	node.shiftPtrAndOffsetRight(insertIdx)
	insertPos += PTR_SIZE + OFFSET_SIZE // update to new position

	// make space for the new key
	totalLen := uint16(len(k) + len(v) + KEY_LEN_SIZE + VAL_LEN_SIZE)
	node.shiftKVRight(totalLen, insertPos)

	// insert kv at that position
	node.putKV(k, v, insertPos)

	// update offset list with the newly added kv pair and also fix other offsets
	insertPos -= node.getHeaderAndMetadataLen() // update insertPos to a relative offset before updating the list
	node.reEvaluateOffsetList(insertIdx, insertPos, totalLen)
}
