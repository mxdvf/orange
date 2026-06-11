// Package engine is the entrypoint into the underlying
// storage engine.
package engine

import (
	"fmt"
	"sync"

	"github.com/mxdvf/orange/internal/btree"
)

type Engine struct {
	btree *btree.BTree
	cache sync.Map
}

type entry struct {
	op int
}

func NewEngine(filename string, sync bool) (*Engine, error) {
	// setup btree
	btree, err := btree.NewBTree(filename, sync)
	if err != nil {
		return nil, fmt.Errorf("failed to setup the btree: %w", err)
	}
	return &Engine{btree: btree}, nil
}

func (eng *Engine) Insert(k, v []byte) error {
	return nil
}

func (eng *Engine) Delete(k, v []byte) error {
	return nil
}

func (eng *Engine) Search(k []byte) ([]byte, error) {
	return nil, nil
}
