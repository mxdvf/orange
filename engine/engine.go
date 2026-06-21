// Package engine is the entrypoint into the underlying
// storage engine.
package engine

import (
	"fmt"
	"time"

	"github.com/mxdvf/orange/btree"
)

type opCode uint16

const (
	insertOp opCode = iota
	deleteOp
)

type writeRequest struct {
	op      opCode
	key     []byte
	value   []byte // since we're supporting both deletes and inserts within this, value can be `nil`
	errChan chan error
}

type Engine struct {
	btree   *btree.BTree
	jobChan chan writeRequest
}

func NewEngine(filename string, sync bool) (*Engine, error) {
	btree, err := btree.NewBTree(filename, sync)
	if err != nil {
		return nil, fmt.Errorf("failed to setup the btree: %w", err)
	}
	eng := &Engine{
		btree:   btree,
		jobChan: make(chan writeRequest, 1000),
	}
	go eng.batchCommitLoop()
	return eng, nil
}

func (eng *Engine) Insert(k, v []byte) error {
	errChan := make(chan error, 1)
	eng.jobChan <- writeRequest{op: insertOp, key: k, value: v, errChan: errChan}
	return <-errChan
}

func (eng *Engine) Delete(k []byte) error {
	errChan := make(chan error, 1)
	eng.jobChan <- writeRequest{op: deleteOp, key: k, errChan: errChan}
	return <-errChan
}

func (eng *Engine) Search(k []byte) ([]byte, error) {
	v, err := eng.btree.Search(k)
	if err != nil {
		return nil, fmt.Errorf("failed to find key: %w", err)
	}
	return v, nil
}

func (eng *Engine) batchCommitLoop() {
	batch := make([]writeRequest, 0, 10)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	// flush helper
	flush := func() {
		if len(batch) == 0 {
			return
		}
		// thread the root pointer through all inserts, each CoW insert
		// returns a new root and the next insert must operate on that root
		root := eng.btree.Root()
		var flushErr error
		for _, req := range batch {
			if flushErr != nil {
				break
			}
			switch req.op {
			case insertOp:
				newRoot, err := eng.btree.InsertNoSync(req.key, req.value, root)
				if err != nil {
					flushErr = fmt.Errorf("insert failed for key %q: %w", req.key, err)
				} else {
					root = newRoot
				}
			case deleteOp:
				newRoot, err := eng.btree.DeleteNoSync(req.key, root)
				if err != nil {
					flushErr = fmt.Errorf("delete failed for key %q: %w", req.key, err)
				} else {
					root = newRoot
				}
			}
		}
		// single fsync for the whole batch — this is the whole point
		if flushErr == nil {
			// fsync barrier 1
			if err := eng.btree.Fsync(); err != nil {
				flushErr = fmt.Errorf("failed to persist the data pages: %w", err)
			}
			// update master page using the pageNum root page
			if err := eng.btree.HandleMasterPage(root); err != nil {
				flushErr = fmt.Errorf("failed to update the master to point to new root: %w", err)
			}
			// fsync barrier 2
			if err := eng.btree.Msync(); err != nil {
				flushErr = fmt.Errorf("failed to persist the master page: %w", err)
			}
		}
		// notify all callers with the shared outcome
		for _, req := range batch {
			req.errChan <- flushErr
		}
		batch = batch[:0]
		ticker.Reset(10 * time.Millisecond)
	}
	// the continous batch commit loop
	for {
		select {
		case req := <-eng.jobChan:
			batch = append(batch, req)
			if len(batch) >= 1000 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
