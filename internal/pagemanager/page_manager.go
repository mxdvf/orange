// Package pagemanager provides low-level page-oriented I/O over a flat file.
// It treats the file as a sequence of fixed-size pages, supporting allocation,
// random reads, and random writes with optional fsync durability.
package pagemanager

import (
	"os"
)

const (
	PreAllocatePageNum = 80 // defines a fixed number by which our file is extended aka 80 pages are allocated at once
)

type PageManager struct {
	file           *os.File
	sync           bool // flag for fsync durability
	maxPageSize    uint32
	currentPageNum uint32
	endPageNum     uint32
}

func NewPageManager(fd *os.File, sync bool, maxPageSize uint32) *PageManager {
	return &PageManager{fd, sync, maxPageSize, 0, 0}
}

func (pm *PageManager) Read(pageNum uint32) ([]byte, error) {
	// given a page number
	start := int64(pageNum * pm.maxPageSize)
	buf := make([]byte, 4096)
	// read the bytes of tha page into the buffer
	_, err := pm.file.ReadAt(buf, start)
	return buf, err
}

func (pm *PageManager) Write(pageNum uint32, buf []byte) error {
	// given a page number and 4096 bytes
	start := int64(pageNum * pm.maxPageSize)
	// write the bytes for that page
	_, err := pm.file.WriteAt(buf, start)
	return err
}

func (pm *PageManager) Fsync() error {
	if pm.sync {
		return pm.file.Sync()
	}
	return nil
}
