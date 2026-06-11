// Package engine is the entrypoint into the underlying
// storage engine.
package engine

import (
	"fmt"

	"github.com/mxdvf/orange/internal/btree"
	"github.com/mxdvf/orange/internal/wal"
)

type Engine struct {
	btree *btree.BTree
	wal   *wal.Wal
}

func NewEngine(filename string, sync bool) (*Engine, error) {
	// setup btree
	btree, err := btree.NewBTree(filename, sync)
	if err != nil {
		return nil, fmt.Errorf("failed to setup the btree: %w", err)
	}
	// setup wal
	wal, err := wal.NewWal(sync)
	if err != nil {
		return nil, fmt.Errorf("failed to setup the wal: %w", err)
	}
	// run a background loop
	// go wal.BackgroundJobLoop(btree)
	// return the engine
	return &Engine{btree: btree, wal: wal}, nil
}

func (eng *Engine) Insert(k, v []byte) error {
	if err := eng.wal.Add(k, v, wal.InsertOp); err != nil {
		return fmt.Errorf("failed to add insert record to the wal: %w", err)
	}
	return nil
}

func (eng *Engine) Delete(k, v []byte) error {
	if err := eng.wal.Add(k, v, wal.DeleteOp); err != nil {
		return fmt.Errorf("failed to add delete record to the wal: %w", err)
	}
	return nil
}

func (eng *Engine) Search(k []byte) ([]byte, error) {
	v, err := eng.wal.Get(k)
	switch err {
	case wal.ErrKeyDoesNotExist:
		return eng.btree.Search(k)
	case wal.ErrKeyDeleted:
		return nil, fmt.Errorf("key does not exist: %w", err)
	}
	return v, nil
}
