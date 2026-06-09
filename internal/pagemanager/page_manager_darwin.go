//go:build darwin

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
	// a chunk of pages using PageAllocateNumBytes
	extendByByteLen := int64(pm.maxPageSize * PreAllocatePageNum)
	// TODO: this is the place causing an error when i attempt to insert
	// 1 million keys to my btree, there's something up with these flags
	// that i need to read about, or maybe first do a clean attempt with
	// the first flag then attempt with the second flag as a fallback
	fstore := unix.Fstore_t{
		Flags:      unix.F_ALLOCATECONTIG | unix.F_ALLOCATEALL,
		Posmode:    unix.F_PEOFPOSMODE,
		Offset:     0, // offset remains 0 because our F_PEOFPOSMODE flag forces allocation to start from EOF
		Length:     extendByByteLen,
		Bytesalloc: 0,
	}
	fd := pm.file.Fd()
	if err := unix.FcntlFstore(fd, unix.F_PREALLOCATE, &fstore); err != nil {
		return 0, fmt.Errorf("failed to allocate space on disk: %w", err)
	}
	// need to truncate the file to update it's size also, only required
	// in darwin as the FcntlFstore syscall only handles block allocation
	currentSize := int64(pm.maxPageSize * pm.endPageNum)
	if err := unix.Ftruncate(int(fd), currentSize+extendByByteLen); err != nil {
		return 0, fmt.Errorf("failed to ftruncate the file metadata: %w", err)
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
	// set the end page num to the last physically allocated block number
	// and also increase the current page num
	pm.endPageNum += PreAllocatePageNum
	currentPageNum := pm.currentPageNum
	pm.currentPageNum++
	return currentPageNum, nil
}

func extendMmap(oldMmapData []byte, file *os.File) ([]byte, error) {
	// unmap the old region
	if err := unix.Munmap(oldMmapData); err != nil {
		return nil, fmt.Errorf("failed to munmap the old region: %w", err)
	}
	// remap the region by initializing a larger mmap'ed region
	data, err := initMmap(file)
	if err != nil {
		return nil, fmt.Errorf("failed to mremap the region: %w", err)
	}
	return data, nil
}
