//go:build linux

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
	// a chunk of pages using PreAllocatePageNum
	fd := int(pm.file.Fd())
	currentByteOffset := int64(pm.currentPageNum * pm.maxPageSize)
	extendByLen := int64(pm.maxPageSize * PreAllocatePageNum)
	if err := unix.Fallocate(fd, 0, currentByteOffset, extendByLen); err != nil {
		return 0, fmt.Errorf("failed to allocate pages: %w", err)
	}
	// shift end page num to last page allocated
	pm.endPageNum += PreAllocatePageNum
	cpn := pm.currentPageNum
	pm.currentPageNum++
	return cpn, nil
}
