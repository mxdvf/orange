// Package btree implements a persistent B-tree backed by a page file
package btree

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type BTree struct {
	root uint32
	pm   *pageManager
}

func NewBTree(filename string) (*BTree, error) {
	// open the main file
	fd, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open the main file: %w", err)
	}
	// initialize root and master pages
	nm := newPageManager(fd)
	root, err := initializeRootAndMasterPage(nm)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize root and master nodes: %v", err)
	}
	// initialize btree
	return &BTree{
		root: root,
		pm:   newPageManager(fd),
	}, nil
}

func initializeRootAndMasterPage(pm *pageManager) (uint32, error) {
	buf, err := pm.read(0)
	switch err {
	// if there's EOF error, then the master page does not exist
	case io.EOF:
		if _, err := pm.allocate(); err != nil {
			return 0, err
		}
		// initialize root page
		root, err := pm.allocate()
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
	// load the root node from disk
	root, err := t.pm.read(t.root)
	if err != nil {
		return fmt.Errorf("failed to load the root node: %w", err)
	}
	// from here, the wrapper takes over (node) and all operations
	// are thus performed on the wrapper
	pageNum, err := t.insertInSubtree(NewNode(root), k, v)
	if err != nil {
		return fmt.Errorf("failed to insert the key: %w", err)
	}
	// update master page using the pageNum root page
	t.pointMasterToNewRoot(pageNum)
	return nil
}

func (t *BTree) insertInSubtree(node *Node, k, v []byte) (uint32, error) {
	// preemptively fix a child before descending into it
	idx, _ := node.findInsertPos(k)
	// 1. find appropriate insertion position
	appropriateSubtreePageNum := node.getPtr(idx)
	// 2. fetch it from disk
	appropriateSubtree, err := t.pm.read(appropriateSubtreePageNum)
	if err != nil {
		return 0, fmt.Errorf("could not read the appropriate subtree's page: %w", err)
	}
	child := NewNode(appropriateSubtree)

	if node.getType() == NODE_TYPE_INTERNAL && child.getSize()+child.getTotalLenPostInsert(k, v) >= PAGE_SIZE {
		newChild, medianKey, medianVal := child.drySplit()
		pageNum, _ := t.copyToNewPage(newChild)
		// 5. fix pointer for the parent
		node.insert(medianKey, medianVal)
		node.setPtr(idx+1, pageNum)
	}

	// TODO: when splitting, the equivalent node must be of the same type
	// TODO: node constructor should have option for setting node type

	switch node.getType() {
	case NODE_TYPE_LEAF:
		// simple logic to insert into the node and perform a
		// copy-on-write
		return t.handleInsertionInLeafNode(node, k, v)
	case NODE_TYPE_INTERNAL:
		// orchestrator logic to enter into the correct subtree page
		// recursively insert on that subtree's root, update all page nums
		// upwards
		return t.handleInsertionInInternalNode(node, k, v)
	}

	panic("should not have reached this point")
}

func (t *BTree) pointMasterToNewRoot(pageNum uint32) error {
	// read master page
	buf, err := t.pm.read(0)
	if err != nil {
		return fmt.Errorf("failed to read the master page: %w", err)
	}
	// update master page pointer to root
	binary.BigEndian.PutUint32(buf[0:], pageNum)
	// write back master page to disk
	if err := t.pm.write(0, buf); err != nil {
		return fmt.Errorf("failed to write to master page: %w", err)
	}
	// also update the in-mem pointer
	t.root = pageNum
	return nil
}

func (t *BTree) handleInsertionInLeafNode(node *Node, k, v []byte) (uint32, error) {
	// attempt insertion on the leaf node
	if err := node.insert(k, v); err != nil {
		return 0, err
	}
	// allocate and write to the new page
	pageNum, err := t.copyToNewPage(node)
	if err != nil {
		return 0, fmt.Errorf("failed to allocate when writing back updated bytes to disk: %w", err)
	}
	// return the newly allocated page num
	return pageNum, nil
}

func (t *BTree) handleInsertionInInternalNode(node *Node, k, v []byte) (uint32, error) {
	// figure out which node it should be
	idx, _ := node.findInsertPos(k)
	appropriateSubtreePageNum := node.getPtr(idx)
	// insert into the appropriate subtree
	appropriateSubtree, err := t.pm.read(appropriateSubtreePageNum)
	if err != nil {
		return 0, fmt.Errorf("could not read the appropriate subtree's page: %w", err)
	}
	childPageNum, err := t.insertInSubtree(NewNode(appropriateSubtree), k, v)
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

func (t *BTree) copyToNewPage(node *Node) (uint32, error) {
	pageNum, err := t.pm.allocate()
	if err != nil {
		return 0, err
	}
	// write the updated bytes to the newly allocated page
	if err := t.pm.write(pageNum, node.data); err != nil {
		return 0, err
	}
	// return the newly allocated page num
	return pageNum, nil
}
