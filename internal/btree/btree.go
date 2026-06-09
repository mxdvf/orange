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

	"github.com/mxdvf/orange/internal/nodemanager"
	"github.com/mxdvf/orange/internal/pagemanager"
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
	root     uint32
	freelist []uint32
	pm       *pagemanager.PageManager
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
	pm, err := pagemanager.NewPageManager(fd, sync, PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize the page manager: %w", err)
	}
	root, err := loadOrCreateRoot(pm)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize root and master nodes: %v", err)
	}
	// initialize btree
	return &BTree{
		root:     root,
		freelist: make([]uint32, 0),
		pm:       pm,
	}, nil
}

func loadOrCreateRoot(pm *pagemanager.PageManager) (uint32, error) {
	buf, err := pm.Read(0)
	switch err {
	// if there's EOF error, then the master page does not exist
	case io.EOF:
		// initialize the master page
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
		rootNode *nodemanager.Node
		err      error
	)
	if rootNode, err = t.loadAsNode(t.root); err != nil {
		return fmt.Errorf("failed to load and transform the root node: %w", err)
	}
	// perform a Split if it's already full
	if rootNode.Overflow() {
		rootNode, err = t.splitRoot(rootNode)
		if err != nil {
			return fmt.Errorf("failed to setup the new root: %w", err)
		}
		// ideally you would want to persist the newly split root here,
		// it does not really matter much because if the insert fails
		// mid-way, the split can just happen again next time
	}
	// from here, the wrapper takes over (node) and all operations
	// are thus performed on the wrapper
	pageNum, err := t.insert(rootNode, k, v)
	if err != nil {
		return fmt.Errorf("failed to insert the key: %w", err)
	}
	// fsync barrier 1, this is because it might be possible that the
	// kernel writes out-of-order as a result the master points to the
	// new root but few of the pages within the tree are still old
	if err := t.pm.Fsync(); err != nil {
		return fmt.Errorf("failed to persist all the newly created pages: %w", err)
	}
	// update master page using the pageNum root page
	t.freelist = append(t.freelist, t.root)
	if err := t.handleMasterPage(pageNum); err != nil {
		return fmt.Errorf("failed to update the master to point to new root: %w", err)
	}
	// fsync barrier 2, as explained above, it's now safe to move the
	// master to point to the new root because at this point we can guarantee
	// that the all data nodes are persisted safely to the disk and that traversing
	// the tree with the new root will be perfeclty alright
	if err := t.pm.Fsync(); err != nil {
		return fmt.Errorf("failed to persist the master page: %w", err)
	}
	return nil
}

func (t *BTree) splitRoot(rootNode *nodemanager.Node) (*nodemanager.Node, error) {
	// Split root into left and right
	left, right, medianIndex := rootNode.Split()
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
	newRootNode := nodemanager.NewNode(buf)
	// set type to internal
	newRootNode.SetType(NodeTypeInternal)
	// insert median key into new root
	medianKey, medianVal := rootNode.GetKV(medianIndex)
	newRootNode.InsertKV(medianKey, medianVal)
	// set pointers to left and right children
	newRootNode.SetPtr(0, leftPageNum)
	newRootNode.SetPtr(1, rightPageNum)
	// return the new root
	return newRootNode, nil
}

func (t *BTree) insert(node *nodemanager.Node, k, v []byte) (uint32, error) {
	// preemptive fix before ever touching a child node
	if node.GetType() == NodeTypeInternal {
		if err := t.splitChild(node, k); err != nil {
			return 0, err
		}
	}
	// handle insertion appropriately
	switch node.GetType() {
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
	panic("should not have reached this point, insert, node can only be internal/leaf")
}

func (t *BTree) splitChild(node *nodemanager.Node, k []byte) error {
	// find the appropriate child that you're about to enter into
	idx := node.FindIndex(k)
	// load that child into a node
	childNode, err := t.loadAsNode(node.GetPtr(idx))
	if err != nil {
		return fmt.Errorf("could not read the appropriate subtree's page: %w", err)
	}
	// check if it's full, if yes break it down into 2 nodes
	if childNode.Overflow() {
		t.freelist = append(t.freelist, node.GetPtr(idx))
		leftChildNode, rightChildNode, medianIndex := childNode.Split()
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
		medianKey, medianVal := childNode.GetKV(medianIndex)
		node.InsertKV(medianKey, medianVal)
		// set pointers to left and right children
		node.SetPtr(idx, leftPageNum)
		node.SetPtr(idx+1, rightPageNum)
	}
	return nil
}

func (t *BTree) insertIntoLeaf(node *nodemanager.Node, k, v []byte) (uint32, error) {
	// TODO: check if the key is already present, if that's the case, simply
	// override the key with the updated value. it needs to be done here.
	// attempt insertion on the leaf node
	node.InsertKV(k, v)
	// allocate and write to the new page
	pageNum, err := t.copyToNewPage(node)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate when writing back updated bytes to disk: %w", err)
	}
	// return the newly allocated page num
	return pageNum, nil
}

func (t *BTree) insertIntoInternal(node *nodemanager.Node, k, v []byte) (uint32, error) {
	// figure out which node it should be
	idx := node.FindIndex(k)
	// TODO: check if the key is already present, if that's the case, simply
	// override the key with the updated value. it needs to be done here.
	// insert into the appropriate subtree

	// this is the child node which will get be abandoned once we recurse into it,
	// and the t.insert returns the new child node page num
	t.freelist = append(t.freelist, node.GetPtr(idx))
	appropriateSubtreeNode, err := t.loadAsNode(node.GetPtr(idx))
	if err != nil {
		return 0, fmt.Errorf("could not read the appropriate subtree's page: %w", err)
	}
	childPageNum, err := t.insert(appropriateSubtreeNode, k, v)
	if err != nil {
		return 0, err
	}
	// we receive the page number of that node and so we now update our pointer
	node.SetPtr(idx, childPageNum)
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
	rootNode := nodemanager.NewNode(root)
	return t.search(rootNode, k)
}

func (t *BTree) search(node *nodemanager.Node, target []byte) ([]byte, error) {
	// this function will give us an index such that target <= some_key_in_node
	idx := node.FindIndex(target)
	// assuming the key is in the node, we will receive an index that is within bounds
	// because if the key is out of bounds then it means that we must traverse down. the
	// only reason we can receive an out of bound index is because we're using the a helper
	// for insertion which can return out of bound index if the key is larger than all keys
	// present in the node
	nKeys := node.GetNKeys()
	if idx < nKeys {
		k, v := node.GetKV(idx)
		if res := bytes.Compare(k, target); res == 0 {
			return v, nil
		}
	}
	// if the node type we're operating on is a leaf, then we end the search
	if node.GetType() == NodeTypeLeaf {
		return nil, fmt.Errorf("reached the end of the tree and couldn't find the key")
	}
	// ptr[idx] is always the correct child because as for pointers, they are always 1 more
	// than the # of keys, so it works for: (1) idx < nkeys, and (2) idx = nkeys because we will
	// always receive the correct pointer
	pageNum := node.GetPtr(idx)
	buf, err := t.pm.Read(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to read page: %v", err)
	}
	// recursively search the subtree
	return t.search(nodemanager.NewNode(buf), target)
}

func (t *BTree) Delete(k []byte) error {
	rootNode, err := t.loadAsNode(t.root)
	if err != nil {
		return fmt.Errorf("failed to load and transform the root node: %w", err)
	}
	pageNum, err := t.delete(rootNode, k)
	if err != nil {
		return fmt.Errorf("failed to delete the key: %w", err)
	}
	// check if root became empty after deletion (root merge collapsed it)
	pageNum, err = t.fillRoot(pageNum)
	if err != nil {
		panic("should have fixed the root here")
	}
	// fsync barrier 1
	if err := t.pm.Fsync(); err != nil {
		return fmt.Errorf("failed to persist the new pages: %w", err)
	}
	// point master to the new root
	t.freelist = append(t.freelist, t.root)
	if err := t.handleMasterPage(pageNum); err != nil {
		return fmt.Errorf("failed to update the master to point to new root: %w", err)
	}
	// fsync barrier 2
	if err := t.pm.Fsync(); err != nil {
		return fmt.Errorf("failed to point master to the new node: %w", err)
	}
	return nil
}

func (t *BTree) delete(node *nodemanager.Node, k []byte) (uint32, error) {
	if node.GetType() == NodeTypeInternal && !node.ContainsKey(k) {
		if err := t.fillChild(node, k); err != nil {
			return 0, err
		}
	}
	switch node.GetType() {
	case NodeTypeLeaf:
		return t.deleteFromLeaf(node, k)
	case NodeTypeInternal:
		return t.deleteFromInternal(node, k)
	}
	panic("should not have reached this point, delete, node can only be internal/leaf")
}

func (t *BTree) deleteFromLeaf(node *nodemanager.Node, k []byte) (uint32, error) {
	// find index where key might be found
	idx := node.FindIndex(k)
	// perform validation if the key even exists in this leaf node
	// because if it does not then key does not exist at all
	if idx >= node.GetNKeys() {
		return 0, ErrKeyNotFound
	}
	existingKey, _ := node.GetKV(idx)
	if !bytes.Equal(existingKey, k) {
		return 0, ErrKeyNotFound
	}
	// perform a deletion in the internal node
	node.DeleteKV(idx)
	pageNum, err := t.copyToNewPage(node)
	if err != nil {
		return 0, err
	}
	return pageNum, nil
}

func (t *BTree) deleteFromInternal(node *nodemanager.Node, k []byte) (uint32, error) {
	idx := node.FindIndex(k)
	// check if this internal node itself contains the key
	if node.ContainsKey(k) {
		return t.deleteKeyFromInternal(node, k, idx)
	}
	// key is not in this node, recurse into appropriate child
	childNode, err := t.loadAsNode(node.GetPtr(idx))
	if err != nil {
		return 0, err
	}
	newChildPageNum, err := t.delete(childNode, k)
	if err != nil {
		return 0, err
	}
	t.freelist = append(t.freelist, node.GetPtr(idx))
	node.SetPtr(idx, newChildPageNum)
	return t.copyToNewPage(node)
}

func (t *BTree) fillChild(node *nodemanager.Node, k []byte) error {
	idx := node.FindIndex(k)
	childNode, err := t.loadAsNode(node.GetPtr(idx))
	if err != nil {
		return fmt.Errorf("could not load child node: %w", err)
	}
	// child is not underflowing, no fix needed
	if !childNode.Underflow() {
		return nil
	}
	// try left sibling rotation first
	if idx > 0 {
		leftSibling, err := t.loadAsNode(node.GetPtr(idx - 1))
		if err != nil {
			return fmt.Errorf("could not load left sibling: %w", err)
		}
		if !leftSibling.Underflow() {
			t.freelist = append(t.freelist, node.GetPtr(idx-1), node.GetPtr(idx))
			return t.rotateRight(node, leftSibling, childNode, idx)
		}
	}
	// try right sibling rotation
	if idx < node.GetNKeys() {
		rightSibling, err := t.loadAsNode(node.GetPtr(idx + 1))
		if err != nil {
			return fmt.Errorf("could not load right sibling: %w", err)
		}
		if !rightSibling.Underflow() {
			t.freelist = append(t.freelist, node.GetPtr(idx), node.GetPtr(idx+1))
			return t.rotateLeft(node, childNode, rightSibling, idx)
		}
	}
	// no rotation possible, merge
	if idx > 0 {
		t.freelist = append(t.freelist, node.GetPtr(idx-1), node.GetPtr(idx))
		return t.mergeChildren(node, idx-1)
	}
	t.freelist = append(t.freelist, node.GetPtr(idx), node.GetPtr(idx+1))
	return t.mergeChildren(node, idx)
}

func (t *BTree) rotateRight(parent, leftSibling, child *nodemanager.Node, idx uint16) error {
	// borrow last key from left sibling
	borrowedKey, borrowedVal := leftSibling.GetKV(leftSibling.GetNKeys() - 1)
	// get the parent separator key (sits at idx-1 between left sibling and child)
	parentKey, parentVal := parent.GetKV(idx - 1)
	// push parent key down into child at position 0
	child.InsertKV(parentKey, parentVal)
	// if left sibling is internal, transfer its rightmost child pointer to child
	if leftSibling.GetType() == NodeTypeInternal {
		danglingPtr := leftSibling.GetPtr(leftSibling.GetNKeys())
		child.SetPtr(0, danglingPtr)
	}
	// replace parent separator with borrowed key, also you're bound to lose the right
	// child of the parent due to the way deletion is modelled but the pointer is being
	// set at the very end because we also need the updated page numbers of the left and
	// current child
	parent.UpdateKV(idx-1, borrowedKey, borrowedVal)
	// remove last key from left sibling
	danglingPtr := leftSibling.GetPtr(leftSibling.GetNKeys() - 1)
	leftSibling.DeleteKV(leftSibling.GetNKeys() - 1)
	leftSibling.SetPtr(leftSibling.GetNKeys(), danglingPtr)
	// persist all three
	leftPageNum, err := t.copyToNewPage(leftSibling)
	if err != nil {
		return err
	}
	childPageNum, err := t.copyToNewPage(child)
	if err != nil {
		return err
	}
	parent.SetPtr(idx-1, leftPageNum)
	parent.SetPtr(idx, childPageNum)
	return nil
}

func (t *BTree) rotateLeft(parent, child, rightSibling *nodemanager.Node, idx uint16) error {
	// borrow first key from right sibling
	borrowedKey, borrowedVal := rightSibling.GetKV(0)
	// get the parent separator key (sits at idx between child and right sibling)
	parentKey, parentVal := parent.GetKV(idx)
	// push parent key down into child at the end
	danglingPtr := child.GetPtr(child.GetNKeys())
	child.InsertKV(parentKey, parentVal)
	child.SetPtr(child.GetNKeys()-1, danglingPtr)
	// if right sibling is internal, transfer its leftmost child pointer to child
	if rightSibling.GetType() == NodeTypeInternal {
		danglingPtr := rightSibling.GetPtr(0)
		child.SetPtr(child.GetNKeys(), danglingPtr)
	}
	// replace parent separator with borrowed key, you're bound to lose the left child
	// but that's fine, read comments in the right rotate method to understand the context
	parent.UpdateKV(idx, borrowedKey, borrowedVal)
	// remove first key from right sibling
	rightSibling.DeleteKV(0)
	// persist all three
	childPageNum, err := t.copyToNewPage(child)
	if err != nil {
		return err
	}
	rightPageNum, err := t.copyToNewPage(rightSibling)
	if err != nil {
		return err
	}
	parent.SetPtr(idx, childPageNum)
	parent.SetPtr(idx+1, rightPageNum)
	return nil
}

func (t *BTree) mergeChildren(parent *nodemanager.Node, idx uint16) error {
	leftChild, err := t.loadAsNode(parent.GetPtr(idx))
	if err != nil {
		return err
	}
	rightChild, err := t.loadAsNode(parent.GetPtr(idx + 1))
	if err != nil {
		return err
	}
	// pull parent separator key down into left child
	parentKey, parentVal := parent.GetKV(idx)
	danglingPtr1 := leftChild.GetPtr(leftChild.GetNKeys())
	leftChild.InsertKV(parentKey, parentVal)
	leftChild.SetPtr(leftChild.GetNKeys()-1, danglingPtr1)
	// empty pointers start here
	emptyPtrIdx := leftChild.GetNKeys()
	// merge right child's keys into left child
	for i := uint16(0); i < rightChild.GetNKeys(); i++ {
		k, v := rightChild.GetKV(i)
		leftChild.InsertKV(k, v)
	}
	// if internal, transfer right child's pointers to left child
	if rightChild.GetType() == NodeTypeInternal {
		for i := uint16(0); i <= rightChild.GetNKeys(); i++ {
			leftChild.SetPtr(emptyPtrIdx, rightChild.GetPtr(i))
			emptyPtrIdx++
		}
	}
	// persist merged left child
	newLeftPageNum, err := t.copyToNewPage(leftChild)
	if err != nil {
		return err
	}
	// remove parent separator key and right child pointer
	parent.DeleteKV(idx)
	parent.SetPtr(idx, newLeftPageNum)
	return nil
}

func (t *BTree) deleteKeyFromInternal(node *nodemanager.Node, k []byte, idx uint16) (uint32, error) {
	// case A1: borrow inorder predecessor from left child
	leftChildNode, err := t.loadAsNode(node.GetPtr(idx))
	if err != nil {
		return 0, err
	}
	if !leftChildNode.Underflow() {
		t.freelist = append(t.freelist, node.GetPtr(idx))
		return t.borrowFromInorderPredecessor(node, leftChildNode, idx)
	}
	// case A2: borrow inorder successor from right child
	rightChildNode, err := t.loadAsNode(node.GetPtr(idx + 1))
	if err != nil {
		return 0, err
	}
	if !rightChildNode.Underflow() {
		t.freelist = append(t.freelist, node.GetPtr(idx+1))
		return t.borrowFromInorderSuccessor(node, rightChildNode, idx)
	}
	// // case A3: both children underflowing, merge and delete
	t.freelist = append(t.freelist, node.GetPtr(idx), node.GetPtr(idx+1))
	if err := t.mergeChildren(node, idx); err != nil {
		return 0, err
	}
	// recurse after merging
	return t.delete(node, k)
}

func (t *BTree) borrowFromInorderPredecessor(node, leftChildNode *nodemanager.Node, idx uint16) (uint32, error) {
	// helper that iteratively finds the predecessor
	inorderPredecessor := func(node *nodemanager.Node) ([]byte, []byte) {
		for node.GetType() != NodeTypeLeaf {
			pageNum := node.GetPtr(node.GetNKeys())
			node, _ = t.loadAsNode(pageNum)
		}
		k, v := node.GetKV(node.GetNKeys() - 1)
		return k, v
	}
	// use the predecessor kv pair
	predKey, predVal := inorderPredecessor(leftChildNode)
	// and replace it with the node's kv at idx
	node.UpdateKV(idx, predKey, predVal)
	// also get rid of the predecessor here itself
	newLeftPageNum, err := t.delete(leftChildNode, predKey)
	if err != nil {
		return 0, err
	}
	node.SetPtr(idx, newLeftPageNum)
	return t.copyToNewPage(node)
}

func (t *BTree) borrowFromInorderSuccessor(node, rightChildNode *nodemanager.Node, idx uint16) (uint32, error) {
	// helper that iteratively finds the successor
	inorderSuccessor := func(node *nodemanager.Node) ([]byte, []byte) {
		for node.GetType() != NodeTypeLeaf {
			pageNum := node.GetPtr(0)
			node, _ = t.loadAsNode(pageNum)
		}
		k, v := node.GetKV(0)
		return k, v
	}
	// use the successor kv pair
	succKey, succVal := inorderSuccessor(rightChildNode)
	// and replace it with the node's kv at idx
	danglingPtr1 := node.GetPtr(idx)
	node.UpdateKV(idx, succKey, succVal)
	node.SetPtr(idx, danglingPtr1)
	// also get rid of the successor here itself
	newRightPageNum, err := t.delete(rightChildNode, succKey)
	if err != nil {
		return 0, err
	}
	node.SetPtr(idx+1, newRightPageNum)
	return t.copyToNewPage(node)
}

func (t *BTree) fillRoot(pageNum uint32) (uint32, error) {
	newRoot, err := t.loadAsNode(pageNum)
	if err != nil {
		return 0, err
	}
	if newRoot.GetNKeys() == 0 && newRoot.GetType() == NodeTypeInternal {
		// root is empty, its only child becomes the new root
		pageNum = newRoot.GetPtr(0)
	}
	return pageNum, nil
}
