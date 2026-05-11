package btree

import (
	"bytes"
	"math/rand"
	"os"
	"testing"
)

func TestNodeManagerAllocate(t *testing.T) {
	fd, err := os.OpenFile("test.bin", os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		t.Fatalf("error opening file: %v", err)
	}

	nm := newNodeManager(fd)
	latestPageNum, err := mockAllocate(nm)
	if err != nil {
		t.Fatalf("failed to allocate a node: %v", err)
	}

	if latestPageNum != 1 {
		t.Fatalf("page number expected: 1, got: %d", latestPageNum)
	}
}

func TestNodeManagerReadWrite(t *testing.T) {
	fd, err := os.OpenFile("test.bin", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		t.Fatalf("error opening file: %v", err)
	}

	nm := newNodeManager(fd)
	mockAllocate(nm)
	pageNum, _ := mockAllocate(nm)

	pageNum = rand.Intn(pageNum + 1)
	t.Logf("attempting to write and then read from page number %d", pageNum)

	buf := make([]byte, 4096)
	target := make([]byte, 4096)
	copy(target[:], "hello")
	copy(buf[:], "hello")

	err = nm.write(pageNum, buf)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	buf1, err := nm.read(pageNum)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	if res := bytes.Compare(target, buf1); res != 0 {
		t.Fatalf("target and retrieved bytes did not match, expected: %v, got: %v, res: %d", string(target), string(buf1), res)
	}
}

func mockAllocate(nm *nodeManager) (int, error) {
	_, err := nm.allocate()
	if err != nil {
		return 0, err
	}
	pageNum, err := nm.allocate()
	if err != nil {
		return 0, err
	}
	return pageNum, nil
}
