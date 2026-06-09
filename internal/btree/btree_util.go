package btree

import (
	"encoding/binary"
	"fmt"

	"github.com/mxdvf/orange/internal/nodemanager"
)

func (t *BTree) print() {
	// performing a standard bfs
	queue := []uint32{t.root}
	level := 0
	for len(queue) != 0 {
		queueLen := len(queue)
		for i := range queueLen {
			pageNum := queue[i]
			if pageNum == 0 {
				fmt.Println("//////////////////////////")
				continue
			}
			buf, _ := t.pm.Read(pageNum)
			n := nodemanager.NewNode(buf)
			// visual print logic
			fmt.Printf("=-----==-----Level: %d-----==-----= (node size: %v)\n", level, n.GetSize())
			fmt.Println(string(n.Data()))
			fmt.Println("=-------==------==------==-------=")
			// only append children if the current node is internal
			if n.GetType() == NodeTypeInternal {
				for idx := range n.GetNKeys() + 1 {
					queue = append(queue, n.GetPtr(idx))
				}
				queue = append(queue, 0)
			}
		}
		// important: shift the queue forward so we don't re-process nodes
		queue = queue[queueLen:]
		level++
	}
}

func (t *BTree) loadAsNode(pageNum uint32) (*nodemanager.Node, error) {
	// load the root node from disk
	root, err := t.pm.Read(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to load the root node: %w", err)
	}
	// transform to node
	return nodemanager.NewNode(root), err
}

func (t *BTree) copyToNewPage(node *nodemanager.Node) (uint32, error) {
	pageNum, err := t.pm.Allocate()
	if err != nil {
		return 0, err
	}
	// write the updated bytes to the newly allocated page
	if err := t.pm.Write(pageNum, node.Data()); err != nil {
		return 0, err
	}
	// return the newly allocated page num
	return pageNum, nil
}

func (t *BTree) pointMasterToNewRoot(pageNum uint32) error {
	// update master page pointer to root
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], pageNum)
	if err := t.pm.Write(0, buf[:]); err != nil {
		return fmt.Errorf("failed to write master page: %w", err)
	}
	// also update the in-mem pointer
	t.root = pageNum
	return nil
}
