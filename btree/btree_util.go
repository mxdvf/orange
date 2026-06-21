package btree

import (
	"encoding/binary"
	"fmt"

	"github.com/mxdvf/orange/nodemanager"
)

func (t *BTree) print(bytemode bool) {
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
			fmt.Printf("=-----==-----Level: %d-----==-----= (node size: %v, page_num: %v)\n", level, n.GetSize(), pageNum)
			if bytemode {
				fmt.Println(n.Data()[:30])
			} else {
				fmt.Println(string(n.Data()))
			}
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

func (t *BTree) HandleMasterPage(pageNum uint32) error {
	// load master into memory
	buf, err := t.pm.Read(0)
	if err != nil {
		return fmt.Errorf("failed to read master page for an RMW cycle: %w", err)
	}
	// step 1: update master page pointer to root
	binary.BigEndian.PutUint32(buf[0:], pageNum)
	// step 2: flush the freelist to the master
	flSize := binary.BigEndian.Uint32(buf[4:])
	// TODO: IMPORTANT! IMPORTANT! IMPORTANT! technically you should maintain an
	// overflow list here, so that in case the number of free pages grow far beyond
	// the fixed number, we still have some way to retrieve them, otherwise they
	// would occupy unnecessary space on the disk. AND THE REASON BEING THAT
	// at some point when you're inserting a ton of keys, the free pages would be
	// producted much faster than you can consume and so you will be ignoring
	// a bunch of them due to the conditional statement below. in that case,
	// almost all benefits of the free page would go away and your db size would
	// keep rising
	// only add items if the freelist has some pages and if it's within limits
	if int(flSize)+len(t.freelist) < (PageSize-MasterHeaderSize)/PointerSize {
		for idx, flItem := range t.freelist {
			start := 8 + int(flSize)*4 + idx*4
			binary.BigEndian.PutUint32(buf[start:], flItem)
		}
		binary.BigEndian.PutUint32(buf[4:], flSize+uint32(len(t.freelist)))
	}
	// TODO: when a write reaches here, at this point, i am not fully confident
	// about my reasoning, although CoW is being followed immaculately and my
	// write throughput is the highest it has reached, my intuition says there's
	// something that i am missing. once done, i should trace the insert request
	// and reason about every side-effect, modification, everything it does,
	// maybe i can squeeze in extra 10-15% performance and uncover some fatal
	// bugs who knows, let's see.
	// write everything at once to the master
	if err := t.pm.Write(0, buf); err != nil {
		return fmt.Errorf("failed to write master page: %w", err)
	}
	// TODO: also it's important to only ever add those pages to the free list which
	// are not being actively traversed by a reader in real-time, that's hard to implement
	// anyways but as soon as you start introducing concurrent workloads, it *would* in
	// failed reads which would be catastrophic given this is meant for read-heavy workloads
	// also update the in-mem pointer and zero-out the freelist
	t.root = pageNum
	t.freelist = make([]uint32, 0)
	return nil
}

func (t *BTree) Root() uint32 {
	return t.root
}

func (t *BTree) Fsync() error {
	return t.pm.Fsync()
}
