//go:build darwin

package pagemanager

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func (pm *PageManager) Allocate() (uint32, error) {
	// if we haven't reached the end of the file, it means we have
	// some disk pages still left to be allocated
	if pm.currentPageNum < pm.endPageNum {
		cpn := pm.currentPageNum
		pm.currentPageNum++
		return cpn, nil
	}
	// since we've reached the end of the file, we allocate a fixed
	// a chunk of pages using PageAllocateNumBytes
	extendByLen := int64(pm.maxPageSize) * PreAllocatePageNum
	// TODO: this is the place causing an error when i attempt to insert
	// 1 million keys to my btree, there's something up with these flags
	// that i need to read about, or maybe first do a clean attempt with
	// the first flag then attempt with the second flag as a fallback
	fstore := unix.Fstore_t{
		Flags:      unix.F_ALLOCATECONTIG | unix.F_ALLOCATEALL,
		Posmode:    unix.F_PEOFPOSMODE,
		Offset:     0,
		Length:     extendByLen,
		Bytesalloc: 0,
	}
	fd := pm.file.Fd()
	if err := unix.FcntlFstore(fd, unix.F_PREALLOCATE, &fstore); err != nil {
		return 0, fmt.Errorf("failed to allocate space on disk: %w", err)
	}
	// need to truncate the file to update it's size also, only required
	// in darwin as the FcntlFstore syscall only handles block allocation
	currentSize := int64(pm.endPageNum * pm.maxPageSize)
	if err := unix.Ftruncate(int(fd), currentSize+extendByLen); err != nil {
		return 0, fmt.Errorf("failed to ftruncate the file metadata: %w", err)
	}
	// set the end page num to the last physically allocated block number
	// and also increase the current page num
	pm.endPageNum += PreAllocatePageNum
	currentPageNum := pm.currentPageNum
	pm.currentPageNum++
	return currentPageNum, nil
}
