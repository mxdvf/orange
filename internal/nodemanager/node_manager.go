package nodemanager

import (
	"bytes"
	"encoding/binary"
)

const (
	MaxAllowedKVLen = 1344 // 1344 bytes, fit 3 keys in a node to maintain b-tree structure

	HeaderSize  = 4 // 2 + 2 = 4 bytes
	PointerSize = 4 // 4 bytes
	OffsetSize  = 2 // 2 bytes
	KeyLenSize  = 2 // 2 bytes
	ValLenSize  = 2 // 2 bytes
)

type Node struct {
	// wire format:
	// type  |  nkeys |   pointers  |  offset-array	 |		     key-values
	//  2B   |   2B   |  nkeys * 4B |  	nkeys * 2B	 |  [klen: 2B][k][vlen: 2B][v]
	data     []byte
	pageSize int
}

// assumptions:
// 1. duplicate keys are rewritten
// 2. offset list contains relative positioning: check kvPos()
// 3. offset list points to the start of the next to-be-inserted KV pair (a logically empty space to start from)

func NewNode(buf []byte) *Node {
	if len(buf) != 4096 {
		panic("we only accept 4096-byte buffers")
	}
	return &Node{data: buf, pageSize: len(buf)}
}

func (node *Node) Data() []byte {
	return node.data
}

func (node *Node) GetSize() uint16 {
	lastIdx := node.GetNKeys() - 1
	kvRelativeLen := node.getOffset(lastIdx) + node.getKVLen(lastIdx)
	return node.getHeaderAndMetadataLen() + kvRelativeLen
}

func (node *Node) GetType() uint16 {
	return binary.BigEndian.Uint16(node.data[0:])
}

func (node *Node) SetType(t uint16) {
	binary.BigEndian.PutUint16(node.data[0:], t)
}

func (node *Node) GetNKeys() uint16 {
	return binary.BigEndian.Uint16(node.data[2:])
}

func (node *Node) incrementNKeys() {
	binary.BigEndian.PutUint16(node.data[2:], node.GetNKeys()+1)
}

func (node *Node) decrementNKeys() {
	binary.BigEndian.PutUint16(node.data[2:], node.GetNKeys()-1)
}

func (node *Node) getHeaderAndMetadataLen() uint16 {
	return HeaderSize + PointerSize*(node.GetNKeys()+1) + OffsetSize*node.GetNKeys()
}

func (node *Node) offsetPos(idx uint16) uint16 {
	return HeaderSize + PointerSize*(node.GetNKeys()+1) + OffsetSize*idx
}

func (node *Node) getOffset(idx uint16) uint16 {
	pos := node.offsetPos(idx)
	return binary.BigEndian.Uint16(node.data[pos:])
}

func (node *Node) setOffset(idx, offset uint16) {
	pos := node.offsetPos(idx)
	binary.BigEndian.PutUint16(node.data[pos:], offset)
}

func (node *Node) kvPos(idx uint16) uint16 {
	if node.GetNKeys() == 0 {
		return HeaderSize + PointerSize*(node.GetNKeys()+1) + OffsetSize*node.GetNKeys() + node.getOffset(0)
	}
	if idx == node.GetNKeys() {
		return node.kvPos(idx-1) + node.getKVLen(idx-1)
	}
	return HeaderSize + PointerSize*(node.GetNKeys()+1) + OffsetSize*node.GetNKeys() + node.getOffset(idx)
}

func (node *Node) GetKV(idx uint16) ([]byte, []byte) {
	// following a seek based approach, we extract what we need
	// and move the seek forward
	pos := node.kvPos(idx)
	// perform action & move forward
	klen := binary.BigEndian.Uint16(node.data[pos:])
	pos += KeyLenSize
	// perform action & move forward
	key := node.data[pos : pos+klen]
	pos += klen
	// perform action & move forward
	vlen := binary.BigEndian.Uint16(node.data[pos:])
	pos += ValLenSize
	// perform action & move forward
	val := node.data[pos : pos+vlen]
	return key, val
}

func (node *Node) setKV(k, v []byte, pos uint16) {
	// a seek based approach
	binary.BigEndian.PutUint16(node.data[pos:], uint16(len(k)))
	pos += KeyLenSize
	// perform action & move forward
	copy(node.data[pos:pos+uint16(len(k))], k)
	pos += uint16(len(k))
	// perform action & move forward
	binary.BigEndian.PutUint16(node.data[pos:], uint16(len(v)))
	pos += ValLenSize
	// perform action
	copy(node.data[pos:pos+uint16(len(v))], v)
}

func (node *Node) UpdateKV(idx uint16, k, v []byte) {
	// simple approach: delete then insert
	// works correctly because UpdateKV is only called when
	// new key has same or similar size (inorder predecessor/successor)
	node.DeleteKV(idx)
	node.InsertKV(k, v)
}

func (node *Node) getKVLen(idx uint16) uint16 {
	pos := node.kvPos(idx)
	klen := binary.BigEndian.Uint16(node.data[pos:])
	vlen := binary.BigEndian.Uint16(node.data[pos+KeyLenSize+klen:])
	return klen + vlen + KeyLenSize + ValLenSize
}

func (node *Node) ptrPos(idx uint16) uint16 {
	return HeaderSize + PointerSize*idx
}

func (node *Node) GetPtr(idx uint16) uint32 {
	pos := HeaderSize + PointerSize*idx
	return binary.BigEndian.Uint32(node.data[pos:])
}

func (node *Node) SetPtr(idx uint16, ptr uint32) {
	pos := HeaderSize + PointerSize*idx
	binary.BigEndian.PutUint32(node.data[pos:], ptr)
}

func (node *Node) getTotalLenIfInserted(k, v []byte) uint16 {
	return uint16(len(k)+len(v)) + OffsetSize + PointerSize + KeyLenSize + ValLenSize
}

func (node *Node) Overflow() bool {
	// a node overflows when it no longer has room for the worst-case key
	// (aka one that's 1344B) that could be promoted from its child into itself
	// during a Split which means a median key of max size. this was a bug that
	// took me 6 days to figure out, although i admit it was an oversight on my part.

	// TODO: parent must be able to look into the child and see if it's median when taken
	// inside of it can cause issues or not. this means that in order to save those 1344B
	// we must do a second preemptive fix on the node (aka parent) we're on right now.
	// this is the only way that we can save those extra bytes, a bit messy and complex
	// but good ROI.
	return int(node.GetSize()) > node.pageSize-MaxAllowedKVLen
}

func (node *Node) Underflow() bool {
	// minimum degree t=2, so minimum keys = t-1 = 1
	// a node underflows when it has only 1 key and we need to delete from it
	return node.GetNKeys() <= 1
}

func (node *Node) FindIndex(target []byte) uint16 {
	if node.GetNKeys() <= 0 {
		return 0
	}
	// loop over all keys to find the appropriate insertion position
	var idx uint16
	for idx = range node.GetNKeys() {
		k, _ := node.GetKV(idx)
		if res := bytes.Compare(target, k); res == -1 || res == 0 {
			return idx
		}
	}
	// for keys that are the largest in the range would be inserted
	// after the last kv pair and hence the offset needs to be calculated
	// manually
	return idx + 1
}

func (node *Node) ContainsKey(k []byte) bool {
	// check if this internal node itself contains the key
	idx := node.FindIndex(k)
	if idx < node.GetNKeys() {
		existingKey, _ := node.GetKV(idx)
		if bytes.Equal(existingKey, k) {
			return true
		}
	}
	return false
}

// ------ below are almost all insertion related methods

func (node *Node) Split() (*Node, *Node, uint16) {
	// check for the median key
	medianIndex := node.GetNKeys() / 2
	// initialize a new node
	leftNode := NewNode(make([]byte, node.pageSize))
	rightNode := NewNode(make([]byte, node.pageSize))
	// set node type
	rightNode.SetType(node.GetType())
	leftNode.SetType(node.GetType())
	// from here on, we will operate on the left half of each component of the node
	// i.e [0, medianIndex), but remember pointers is always 1 more than nkeys
	for idx := uint16(0); idx < medianIndex; idx++ {
		k, v := node.GetKV(idx)
		leftNode.InsertKV(k, v)
	}
	// pointer always goes one more than the nunber of keys, hence different loop
	for idx := uint16(0); idx < medianIndex+1; idx++ {
		leftNode.SetPtr(idx, node.GetPtr(idx))
	}
	// from here, we will operate on the right half of each component of the node
	// i.e [medianIndex+1, nkeys), but again pointers is always 1 more than nkeys
	for idx := medianIndex + 1; idx < node.GetNKeys(); idx++ {
		k, v := node.GetKV(idx)
		rightNode.InsertKV(k, v)
	}
	for idx := medianIndex + 1; idx < node.GetNKeys()+1; idx++ {
		rightNode.SetPtr(idx-medianIndex-1, node.GetPtr(idx))
	}
	// return
	return leftNode, rightNode, medianIndex
}

func (node *Node) InsertKV(k, v []byte) {
	if node.GetSize()+node.getTotalLenIfInserted(k, v) >= uint16(node.pageSize) {
		panic("illegal node, it should have been Split by a preemptive fix")
	}
	// find insertion point
	insertIdx := node.FindIndex(k)
	insertPos := node.kvPos(insertIdx)
	// increment nkeys
	node.incrementNKeys()
	// make space for ptr, offset, then kv
	kvLen := uint16(len(k) + len(v) + KeyLenSize + ValLenSize)
	node.shiftRight(insertIdx, insertPos, kvLen)
	// put kv there
	insertPos += PointerSize + OffsetSize
	node.setKV(k, v, insertPos)
	// update insertPos to a relative offset before updating the list and
	// fix offset for the node and nodes after it
	insertPos -= node.getHeaderAndMetadataLen()
	node.setOffset(insertIdx, insertPos)
	node.updateOffsets1(insertIdx, kvLen)
}

func (node *Node) shiftRight(idx, pos, kvLen uint16) {
	// make space for new kv's pointer
	ptrPos := node.ptrPos(idx)
	copy(node.data[ptrPos+PointerSize:], node.data[ptrPos:])
	clear(node.data[ptrPos : ptrPos+PointerSize])
	// make space for new kv's offset
	offsetPos := node.offsetPos(idx)
	copy(node.data[offsetPos+OffsetSize:], node.data[offsetPos:])
	clear(node.data[offsetPos : offsetPos+OffsetSize])
	// make space for the kv pair
	pos += PointerSize + OffsetSize
	copy(node.data[pos+kvLen:], node.data[pos:])
	clear(node.data[pos : pos+kvLen])
}

func (node *Node) updateOffsets1(idx, totalLen uint16) {
	for i := idx + 1; i < node.GetNKeys(); i++ {
		node.setOffset(i, node.getOffset(i)+totalLen)
	}
}

// ------- below are almost all deletion related methods

func (node *Node) DeleteKV(idx uint16) {
	// find deletion point
	kvLen, kvStart := node.getKVLen(idx), node.kvPos(idx)
	nodeSize := node.GetSize()
	// remove kv by shifting everything left
	copy(node.data[kvStart:], node.data[kvStart+kvLen:])
	clear(node.data[nodeSize-kvLen:])
	nodeSize -= kvLen
	// remove offset at idx and update remaining offsets
	offsetPos := node.offsetPos(idx)
	copy(node.data[offsetPos:], node.data[offsetPos+OffsetSize:])
	clear(node.data[nodeSize-OffsetSize:])
	nodeSize -= OffsetSize
	// remove pointer at idx
	ptrPos := node.ptrPos(idx)
	copy(node.data[ptrPos:], node.data[ptrPos+PointerSize:])
	clear(node.data[nodeSize-PointerSize:])
	// decrement nkeys
	node.decrementNKeys()
	// fix offsets for all keys after idx
	node.updateOffsets2(idx, kvLen)
}

func (node *Node) updateOffsets2(idx, kvLen uint16) {
	for i := idx; i < node.GetNKeys(); i++ {
		node.setOffset(i, node.getOffset(i)-kvLen)
	}
}
