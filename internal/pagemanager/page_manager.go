// Package pagemanager provides low-level page-oriented I/O over a flat file.
// It treats the file as a sequence of fixed-size pages, supporting allocation,
// random reads, and random writes with optional fsync durability.
package pagemanager

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

const (
	PreAllocatePageNum = 80 // defines a fixed number by which our file is extended aka 80 pages are allocated at once
)

var (
	ErrFreeListEmpty error = errors.New("free list is empty, fallback to physical disk allocation")
)

type PageManager struct {
	file           *os.File // pointer to the main file
	sync           bool     // flag for fsync durability
	mmapData       []byte   // mmap'ed region in kernel page cache
	maxPageSize    uint32   // the page size we intend to use for our engine (eg: 4096-byte pages)
	currentPageNum uint32   // points to an empty page that is already pre-allocated
	endPageNum     uint32   // points to the last page we have as a pre-allocated page
}

func NewPageManager(file *os.File, sync bool, maxPageSize uint32) (*PageManager, error) {
	// TODO: major flaw here, if i run my engine once which generates
	// the main file, i should be able to run my engine again using
	// the same file. currently it won't work because if initialize
	// my btree on the same file again, it will consequently intiialize
	// the page manager too and at that point the page manager would have
	// lost all counts of the pages it had allocated so meticulously,
	// the ONLY WAY TO FIX this is by persisting currentPageNum and
	// endPageNum in the master page too, otherwise there's no way
	// of knowing AFTER A CRASH OR REBOOT which page we were on
	// and how many pages did we allocate causing our main file to massively
	// blow up in size.
	//
	// TODO: the way to fix this would be to read the file and
	// encounter all the empty allocated pages that have nkeys == 0, and rebuild
	// the currentPageNum and endPageNum, this would work because the contiguous
	// chunks will always be at the tail of the file because of how we prioritize
	// free list
	return &PageManager{file, sync, nil, maxPageSize, 0, 0}, nil
}

func (pm *PageManager) Read(pageNum uint32) ([]byte, error) {
	// edge case meaning the engine is attempting to read the master and root
	// pages because most probably it's restarting and setting everyting up
	if pm.endPageNum == 0 && pageNum == 0 {
		return nil, io.EOF
	}
	// given a page number, fetch the exact portion from the
	// mmap'ed region by using a slicing mechanism
	start := pageNum * pm.maxPageSize
	data := pm.mmapData[start : start+pm.maxPageSize]
	// copy onto a new buffer. MAJOR OVERSIGHT, if mmap allows
	// me to r/w directly into the kernel page cache, then it
	// breaks the very integrity of my CoW semantics
	buf := make([]byte, pm.maxPageSize)
	copy(buf, data)
	return buf, nil
}

func (pm *PageManager) Write(pageNum uint32, buf []byte) error {
	// given a page number, calculate the offset at which the
	// write must begin to happen in the mmap'ed region
	start := pageNum * pm.maxPageSize
	copy(pm.mmapData[start:start+pm.maxPageSize], buf)
	return nil
}

func (pm *PageManager) Fsync() error {
	if pm.sync {
		return pm.file.Sync()
	}
	return nil
}

func (pm *PageManager) MsyncMaster() error {
	return unix.Msync(pm.mmapData[:4096], unix.MS_SYNC)
}

func (pm *PageManager) allocateViaFreeList() (uint32, error) {
	// retrieve the size of free list
	flSize := binary.BigEndian.Uint32(pm.mmapData[4:])
	// if free list is empty, inform the caller to fallback
	if flSize <= 0 {
		return 0, ErrFreeListEmpty
	}
	// if free list has some pages, retrieve the first one
	// from the head
	pageNum := binary.BigEndian.Uint32(pm.mmapData[8:])
	copy(pm.mmapData[8:], pm.mmapData[12:])
	clear(pm.mmapData[len(pm.mmapData)-4:])
	// and reduce the free list size count
	binary.BigEndian.PutUint32(pm.mmapData[4:], flSize-1)
	if err := pm.MsyncMaster(); err != nil {
		return 0, fmt.Errorf("failed to msync the master page: %w", err)
	}
	return pageNum, nil
}

func filelength(file *os.File) (int, error) {
	// get the file size for param in mmap
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %v", err)
	}
	length := int(fileInfo.Size())
	return length, nil
}

func initMmap(file *os.File) ([]byte, error) {
	// fetch the length of the file
	length, err := filelength(file)
	if err != nil {
		return nil, fmt.Errorf("failed to get back the length of the file: %w", err)
	}
	// memory-map the file and use protections PROT_READ | PROT_WRITE
	// for r/w access to the mapped memory slice, and use
	// flag MAP_SHARED which ensures modifications are written
	// back to the underlying file
	data, err := unix.Mmap(
		int(file.Fd()),
		0,
		length,
		unix.PROT_READ|unix.PROT_WRITE,
		unix.MAP_SHARED,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to mmap: %v", err)
	}
	return data, nil
}
