package pagemanager

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
)

const (
	MockPageSize = 4096
	MockSync     = false
)

func init() {
	err := os.MkdirAll("test/", 0755)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setup(t *testing.T) *PageManager {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	fd, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if t != nil {
		t.Logf("running test case for file: %v", filename)
	}
	if err != nil {
		t.Fatalf("error opening file: %v", err)
	}
	return NewPageManager(fd, MockSync, MockPageSize)
}

func TestPageManagerAllocate(t *testing.T) {
	pm := setup(t)
	latestPageNum, err := mockAllocateTwice(pm)
	if err != nil {
		t.Fatalf("failed to allocate a page: %v", err)
	}

	if latestPageNum != 1 {
		t.Fatalf("page number expected: 1, got: %d", latestPageNum)
	}

	info, err := pm.file.Stat()
	if err != nil {
		panic(err)
	}
	fileLen := int(info.Size() / int64(pm.maxPageSize))

	if fileLen != PreAllocatePageNum {
		t.Fatalf("total pages expected: %d, got: %d", PreAllocatePageNum, fileLen)
	}
}

func TestPageManagerReadWrite(t *testing.T) {
	pm := setup(t)

	mockAllocateTwice(pm)
	pageNum, _ := mockAllocateTwice(pm)

	pageNum = rand.Intn(pageNum + 1)
	t.Logf("attempting to write and then read from page number %d", pageNum)

	buf := make([]byte, 4096)
	target := make([]byte, 4096)
	copy(target[:], "hello")
	copy(buf[:], "hello")

	err := pm.Write(uint32(pageNum), buf)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	buf1, err := pm.Read(uint32(pageNum))
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	if res := bytes.Compare(target, buf1); res != 0 {
		t.Fatalf("target and retrieved bytes did not match, expected: %v, got: %v, res: %d", string(target), string(buf1), res)
	}
}

// pages are sequential and non-overlapping
func TestAllocateSequential(t *testing.T) {
	pm := setup(t)
	seen := map[uint32]bool{}
	for range 100 {
		pageNum, err := pm.Allocate()
		if err != nil {
			t.Fatal(err)
		}
		if seen[pageNum] {
			t.Fatalf("duplicate page number returned: %d", pageNum)
		}
		seen[pageNum] = true
	}
}

// data written to a page survives across the chunk boundary
// (this is where your bug would have been caught)
func TestAllocateDataSurvivesGrowth(t *testing.T) {
	pm := setup(t)
	// write to last page before chunk boundary
	lastBeforeGrowth := PreAllocatePageNum - 1
	data := []byte(strings.Repeat("X", 4096))
	if err := pm.Write(uint32(lastBeforeGrowth), data); err != nil {
		t.Fatal(err)
	}
	// force growth past the chunk boundary
	for range PreAllocatePageNum + 1 {
		if _, err := pm.Allocate(); err != nil {
			t.Fatal(err)
		}
	}
	// read it back
	got, err := pm.Read(uint32(lastBeforeGrowth))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("data corrupted across chunk boundary")
	}
}

// allocating exactly at chunk boundaries doesn't skip or duplicate
func TestAllocateChunkBoundary(t *testing.T) {
	pm := setup(t)
	prev := uint32(0)
	for i := range PreAllocatePageNum * 3 {
		pageNum, err := pm.Allocate()
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && pageNum != prev+1 {
			t.Fatalf("gap or duplicate at boundary: got %d, want %d", pageNum, prev+1)
		}
		prev = pageNum
	}
}

func mockAllocateTwice(pm *PageManager) (int, error) {
	_, err := pm.Allocate()
	if err != nil {
		return 0, err
	}
	pageNum, err := pm.Allocate()
	if err != nil {
		return 0, err
	}
	return int(pageNum), nil
}
