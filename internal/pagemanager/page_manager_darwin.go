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
		pm.currentPageNum++
		return pm.currentPageNum, nil
	}
	// since we've reached the end of the file, we allocate a fixed
	// a chunk of pages using PageAllocateNumBytes
	extendByLen := int64(pm.maxPageSize) * PreAllocatePageNum
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
	// fmt.Println("these many bytes were allocated at once", fstore.Bytesalloc, fstore.Bytesalloc/4096)
	// need to truncate the file to update it's size also, only required
	// in darwin as the FcntlFstore syscall only handles block allocation
	if err := unix.Ftruncate(int(fd), extendByLen); err != nil {
		return 0, fmt.Errorf("failed to ftruncate the file metadata: %w", err)
	}
	// set the end page num to the last physically allocated block number
	// and also increase the current page num
	pm.endPageNum += PreAllocatePageNum
	pm.currentPageNum++
	return pm.currentPageNum, nil
}
