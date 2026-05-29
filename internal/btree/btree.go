// Package btree implements a persistent, copy-on-write B-tree backed by a
// page-managed file. Each write operation allocates a new page rather than
// mutating in place, giving the tree append-only, crash-safe semantics.
package btree

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mxdvf/superfastkv/internal/pagemanager"
)

const (
	NodeTypeLeaf uint16 = iota
	NodeTypeInternal
)

const (
	MaxAllowedKVLen = 1344 // 1344 bytes, fit 3 keys in a node to maintain b-tree structure

	PageSize    = 4096 // 4096 bytes
	HeaderSize  = 4    // 2 + 2 = 4 bytes
	PointerSize = 4    // 4 bytes
	OffsetSize  = 2    // 2 bytes
	KeyLenSize  = 2    // 2 bytes
	ValLenSize  = 2    // 2 bytes
)

var (
	ErrOverflow    = errors.New("key+value too large")
	ErrKeyNotFound = errors.New("key not found")
)

type BTree struct {
	root uint32
	pm   *pagemanager.PageManager
}

func NewBTree(filename string, sync bool) (*BTree, error) {
	// open the main file
	fd, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open the main file: %w", err)
	}
	// TODO: need to expose a Close() method to the user so they could
	// defer closing the file in their program, although we're using
	// fsync but it's still should be a safe practice to provide this
	// initialize root and master pages
	pm := pagemanager.NewPageManager(fd, PageSize, sync)
	root, err := loadOrCreateRoot(pm)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize root and master nodes: %v", err)
	}
	// initialize btree
	return &BTree{
		root: root,
		pm:   pm,
	}, nil
}

func loadOrCreateRoot(pm *pagemanager.PageManager) (uint32, error) {
	buf, err := pm.Read(0)
	switch err {
	// if there's EOF error, then the master page does not exist
	case io.EOF:
		if _, err := pm.Allocate(); err != nil {
			return 0, err
		}
		// initialize root page
		root, err := pm.Allocate()
		return root, err
	// if there's no error then master page exists --> root page also exists
	case nil:
		rootPageNum := binary.BigEndian.Uint32(buf[0:])
		return rootPageNum, nil
	// if there's some error, abort
	default:
		return 0, err
	}
}

func (t *BTree) Insert(k, v []byte) error {
	// validate
	if len(k)+len(v) > MaxAllowedKVLen {
		return ErrOverflow
	}
	// load the root from disk
	var (
		rootNode *Node
		err      error
	)
	if rootNode, err = t.loadAsNode(t.root); err != nil {
		return fmt.Errorf("failed to load and transform the root node: %w", err)
	}
	// perform a split if it's already full
	if rootNode.overflow() {
		rootNode, err = t.splitRoot(rootNode)
		if err != nil {
			return fmt.Errorf("failed to setup the new root: %w", err)
		}
	}
	// from here, the wrapper takes over (node) and all operations
	// are thus performed on the wrapper
	pageNum, err := t.insert(rootNode, k, v)
	if err != nil {
		return fmt.Errorf("failed to insert the key: %w", err)
	}
	// update master page using the pageNum root page
	t.pointMasterToNewRoot(pageNum)

	return nil
}

func (t *BTree) splitRoot(rootNode *Node) (*Node, error) {
	// split root into left and right
	left, right, medianIndex := rootNode.split()
	// persist the right node to disk
	rightPageNum, err := t.copyToNewPage(right)
	if err != nil {
		return nil, fmt.Errorf("could not persist the right node while splitting root: %w", err)
	}
	// persist left child
	leftPageNum, err := t.copyToNewPage(left)
	if err != nil {
		return nil, fmt.Errorf("could not persist the left node while splitting root: %w", err)
	}
	// create new root
	buf := make([]byte, PageSize)
	newRootNode := NewNode(buf)
	// set type to internal
	newRootNode.setType(NodeTypeInternal)
	// insert median key into new root
	medianKey, medianVal := rootNode.getKV(medianIndex)
	newRootNode.insertKV(medianKey, medianVal)
	// set pointers to left and right children
	newRootNode.setPtr(0, leftPageNum)
	newRootNode.setPtr(1, rightPageNum)
	// return the new root
	return newRootNode, nil
}

func (t *BTree) insert(node *Node, k, v []byte) (uint32, error) {
	// preemptive fix before ever touching a child node
	if node.getType() == NodeTypeInternal {
		if err := t.splitChild(node, k, v); err != nil {
			return 0, err
		}
	}
	// handle insertion appropriately
	switch node.getType() {
	case NodeTypeLeaf:
		// simple logic to insert into the node and perform a
		// copy-on-write
		return t.insertIntoLeaf(node, k, v)
	case NodeTypeInternal:
		// orchestrator logic to enter into the correct subtree page
		// recursively insert on that subtree's root, update all page nums
		// upwards
		return t.insertIntoInternal(node, k, v)
	}
	panic("should not have reached this point")
}

func (t *BTree) splitChild(node *Node, k, v []byte) error {
	// find the appropriate child that you're about to enter into
	idx := node.findIndex(k)
	// load that child into a node
	childNode, err := t.loadAsNode(node.getPtr(idx))
	if err != nil {
		return fmt.Errorf("could not read the appropriate subtree's page: %w", err)
	}
	// check if it's full, if yes break it down into 2 nodes
	if childNode.overflow() {
		leftChildNode, rightChildNode, medianIndex := childNode.split()
		// persist the right node to disk
		rightPageNum, err := t.copyToNewPage(rightChildNode)
		if err != nil {
			return fmt.Errorf("could not persist the right node during preemptive fix: %w", err)
		}
		// persist left child
		leftPageNum, err := t.copyToNewPage(leftChildNode)
		if err != nil {
			return fmt.Errorf("could not persist the left node during preemptive fix: %w", err)
		}
		// insert median key into new root
		medianKey, medianVal := childNode.getKV(medianIndex)
		node.insertKV(medianKey, medianVal)
		// set pointers to left and right children
		node.setPtr(idx, leftPageNum)
		node.setPtr(idx+1, rightPageNum)
	}
	return nil
}

func (t *BTree) insertIntoLeaf(node *Node, k, v []byte) (uint32, error) {
	// attempt insertion on the leaf node
	node.insertKV(k, v)
	// allocate and write to the new page
	pageNum, err := t.copyToNewPage(node)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate when writing back updated bytes to disk: %w", err)
	}
	// return the newly allocated page num
	return pageNum, nil
}

func (t *BTree) insertIntoInternal(node *Node, k, v []byte) (uint32, error) {
	// figure out which node it should be
	idx := node.findIndex(k)
	// insert into the appropriate subtree
	appropriateSubtree, err := t.loadAsNode(node.getPtr(idx))
	if err != nil {
		return 0, fmt.Errorf("could not read the appropriate subtree's page: %w", err)
	}
	childPageNum, err := t.insert(appropriateSubtree, k, v)
	if err != nil {
		return 0, err
	}
	// we receive the page number of that node and so we now update our pointer
	node.setPtr(idx, childPageNum)
	// this node itself is put to a new location
	pageNum, err := t.copyToNewPage(node)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate when writing back updated bytes to disk: %w", err)
	}
	// and finally we return the pagenum of this
	return pageNum, nil
}

func (t *BTree) Search(k []byte) ([]byte, error) {
	root, err := t.pm.Read(t.root)
	if err != nil {
		return nil, fmt.Errorf("failed to read the root: %v", err)
	}
	rootNode := NewNode(root)
	return t.search(rootNode, k)
}

func (t *BTree) search(node *Node, target []byte) ([]byte, error) {
	// this function will give us an index such that target <= some_key_in_node
	idx := node.findIndex(target)
	// assuming the key is in the node, we will receive an index that is within bounds
	// because if the key is out of bounds then it means that we must traverse down. the
	// only reason we can receive an out of bound index is because we're using the a helper
	// for insertion which can return out of bound index if the key is larger than all keys
	// present in the node
	nKeys := node.getNKeys()
	if idx < nKeys {
		k, v := node.getKV(idx)
		if res := bytes.Compare(k, target); res == 0 {
			return v, nil
		}
	}
	// if the node type we're operating on is a leaf, then we end the search
	if node.getType() == NodeTypeLeaf {
		return nil, fmt.Errorf("reached the end of the tree and couldn't find the key")
	}
	// ptr[idx] is always the correct child because as for pointers, they are always 1 more
	// than the # of keys, so it works for: (1) idx < nkeys, and (2) idx = nkeys because we will
	// always receive the correct pointer
	pageNum := node.getPtr(idx)
	buf, err := t.pm.Read(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to read page: %v", err)
	}
	// recursively search the subtree
	return t.search(NewNode(buf), target)
}

func (t *BTree) Delete(k []byte) error {
	rootNode, err := t.loadAsNode(t.root)
	if err != nil {
		return fmt.Errorf("failed to load and transform the root node: %w", err)
	}
	pageNum, err := t.delete(rootNode, k)
	if err != nil {
		return fmt.Errorf("failed to insert the key: %w", err)
	}
	t.pointMasterToNewRoot(pageNum)
	return nil
}

func (t *BTree) delete(node *Node, k []byte) (uint32, error) {
	// if node.getType() == NodeTypeInternal {
	// 	if err := t.preemptiveDeleteFix(node, k); err != nil {
	// 		return 0, err
	// 	}
	// }
	switch node.getType() {
	case NodeTypeLeaf:
		return t.deleteFromLeaf(node, k)
	case NodeTypeInternal:
		return t.deleteFromInternal(node, k)
	}
	panic("should not have reached this point")
}

func (t *BTree) deleteFromLeaf(node *Node, k []byte) (uint32, error) {
	// find index where key might be found
	idx := node.findIndex(k)
	// perform validation if the key even exists in this leaf node
	// because if it does not then key does not exist at all
	if idx >= node.getNKeys() {
		return 0, ErrKeyNotFound
	}
	existingKey, _ := node.getKV(idx)
	if !bytes.Equal(existingKey, k) {
		return 0, ErrKeyNotFound
	}
	// perform a deletion in the internal node
	node.deleteKV(idx)
	pageNum, err := t.copyToNewPage(node)
	if err != nil {
		return 0, err
	}
	return pageNum, nil
}

func (t *BTree) deleteFromInternal(node *Node, k []byte) (uint32, error) {
	idx := node.findIndex(k)
	// check if this internal node itself contains the key
	if idx < node.getNKeys() {
		existingKey, _ := node.getKV(idx)
		if bytes.Equal(existingKey, k) {
			return t.deleteKeyFromInternal(node, k, idx)
		}
	}
	// key is not in this node, recurse into appropriate child
	childPageNum := node.getPtr(idx)
	childNode, err := t.loadAsNode(childPageNum)
	if err != nil {
		return 0, err
	}
	newChildPageNum, err := t.delete(childNode, k)
	if err != nil {
		return 0, err
	}
	node.setPtr(idx, newChildPageNum)
	return t.copyToNewPage(node)
}

func (t *BTree) deleteKeyFromInternal(node *Node, k []byte, idx uint16) (uint32, error) {
	// case A1: borrow inorder predecessor from left child
	leftChildNode, err := t.loadAsNode(node.getPtr(idx))
	if err != nil {
		return 0, err
	}
	if !leftChildNode.underflow() {
		return t.borrowFromInorderPredecessor(node, leftChildNode, idx)
	}
	// case A2: borrow inorder successor from right child
	rightChildNode, err := t.loadAsNode(node.getPtr(idx + 1))
	if err != nil {
		return 0, err
	}
	if !rightChildNode.underflow() {
		return t.borrowFromInorderSuccessor(node, rightChildNode, idx)
	}
	// // // case A3: both children underflowing, merge and delete
	// // if err := t.mergeChildren(node, idx); err != nil {
	// // 	return 0, err
	// // }
	// recurse after merging
	return t.delete(node, k)
}

func (t *BTree) borrowFromInorderPredecessor(node, leftChildNode *Node, idx uint16) (uint32, error) {
	// helper that iteratively finds the predecessor
	inorderPredecessor := func(node *Node) ([]byte, []byte) {
		for node.getType() != NodeTypeLeaf {
			pageNum := node.getPtr(node.getNKeys())
			node, _ = t.loadAsNode(pageNum)
		}
		k, v := node.getKV(node.getNKeys() - 1)
		return k, v
	}
	// use the predecessor kv pair
	predecessor, _ := inorderPredecessor(leftChildNode)
	// and replace it with the node's kv at idx
	// node.updateKV(idx, predecessor, predVal) TODO: fix this next
	// also get rid of the predecessor here itself
	newLeftPageNum, err := t.delete(leftChildNode, predecessor)
	if err != nil {
		return 0, err
	}
	node.setPtr(idx, newLeftPageNum)
	return t.copyToNewPage(node)
}

func (t *BTree) borrowFromInorderSuccessor(node, rightChildNode *Node, idx uint16) (uint32, error) {
	// helper that iteratively finds the successor
	inorderSuccessor := func(node *Node) ([]byte, []byte) {
		for node.getType() != NodeTypeLeaf {
			pageNum := node.getPtr(0)
			node, _ = t.loadAsNode(pageNum)
		}
		k, v := node.getKV(0)
		return k, v
	}
	// use the successor kv pair
	successor, _ := inorderSuccessor(rightChildNode)
	// and replace it with the node's kv at idx
	// node.updateKV(idx, successor, succVal) TODO: fix this next
	// also get rid of the successor here itself
	newRightPageNum, err := t.delete(rightChildNode, successor)
	if err != nil {
		return 0, err
	}
	node.setPtr(idx+1, newRightPageNum)
	return t.copyToNewPage(node)
}
