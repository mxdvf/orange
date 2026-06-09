//go:build linux

package pagemanager

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func (pm *PageManager) Allocate() (uint32, error) {
	// attempt to serve the allocate request via the freelist
	if pm.endPageNum != 0 {
		pageNum, err := pm.allocateViaFreeList()
		if err == nil {
			return pageNum, nil
		}
		if !errors.Is(err, ErrFreeListEmpty) {
			return 0, fmt.Errorf("failed to receive page num from free list, some error occurred: %w", err)
		}
	}
	// if free-list couldn't serve the allocation request, then we
	// need to request physical pages from the disk. if we haven't
	// reached the end of the file, it means we have, some disk pages
	// still left to be allocated
	if pm.currentPageNum < pm.endPageNum {
		cpn := pm.currentPageNum
		pm.currentPageNum++
		return cpn, nil
	}
	// since we've reached the end of the file, we allocate a fixed
	// a chunk of pages using PreAllocatePageNum
	fd := int(pm.file.Fd())
	currentByteOffset := int64(pm.maxPageSize * pm.currentPageNum)
	extendByByteLen := int64(pm.maxPageSize * PreAllocatePageNum)
	if err := unix.Fallocate(fd, 0, currentByteOffset, extendByByteLen); err != nil {
		return 0, fmt.Errorf("failed to allocate pages: %w", err)
	}
	// since the allocation has happened irrespective of it being the first
	// or subsequent times, we can now set up our mmap region either by
	// initializing it for the first time or remapping a known region
	switch pm.endPageNum {
	case 0:
		var err error
		pm.mmapData, err = initMmap(pm.file)
		if err != nil {
			return 0, fmt.Errorf("failed to initialize the mmap region: %w", err)
		}
	default:
		var err error
		pm.mmapData, err = extendMmap(pm.mmapData, pm.file)
		if err != nil {
			return 0, fmt.Errorf("failed to unmap and mmap to a new region: %w", err)
		}
	}
	// shift end page num to last page allocated
	pm.endPageNum += PreAllocatePageNum
	cpn := pm.currentPageNum
	pm.currentPageNum++
	return cpn, nil
}

func extendMmap(oldMmapData []byte, file *os.File) ([]byte, error) {
	// fetch the length of the file
	length, err := filelength(file)
	if err != nil {
		return nil, fmt.Errorf("failed to get back the length of the file: %w", err)
	}
	// remap the entire mmap'ed region using the mremap(2) syscall
	data, err := unix.Mremap(oldMmapData, length, unix.MREMAP_MAYMOVE)
	if err != nil {
		return nil, fmt.Errorf("failed to mremap the region: %w", err)
	}
	return data, nil
}
