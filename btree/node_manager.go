package btree

import (
	"io"
	"os"
)

type nodeManager struct {
	file *os.File
}

func newNodeManager(fd *os.File) *nodeManager {
	return &nodeManager{fd}
}

// allocate: create a new empty page and return the page number
func (nm *nodeManager) allocate() (int, error) {
	buf := make([]byte, 4096)
	nm.file.Write(buf)
	offset, err := nm.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	return int(offset/4096) - 1, nil
}

// read: given a page number, return the 4096 bytes
func (nm *nodeManager) read(pageNum int) ([]byte, error) {
	start := int64(pageNum * PAGE_SIZE)
	buf := make([]byte, 4096)
	_, err := nm.file.ReadAt(buf, start)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// write: given a page number and 4096 bytes, it just persists
func (nm *nodeManager) write(pageNum int, buf []byte) error {
	start := int64(pageNum * PAGE_SIZE)
	_, err := nm.file.WriteAt(buf, start)
	if err != nil {
		return err
	}
	return nil
}
