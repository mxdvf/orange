package wal

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestWalRecordFormat(t *testing.T) {
	buf := prepareRecord([]byte("mehul"), []byte("kacky"), 2)
	finalBuf := []byte{0, 2, 0, 5, 109, 101, 104, 117, 108, 0, 5, 107, 97, 99, 107, 121}
	if !bytes.Equal(buf, finalBuf) {
		t.Fatal("failed to match up bytes, something wrong with record format")
	}
}

func TestWalAddRecordOnce(t *testing.T) {
	wal, _ := NewWal()

	start := time.Now()
	if err := wal.Add([]byte("mehul"), []byte("kacky"), 2); err != nil {
		t.Fatalf("could not insert: %v", err)
	}
	t.Log(time.Since(start))
}

func TestWalAddRecordMultiple(t *testing.T) {
	wal, _ := NewWal()
	start := time.Now()
	for i := range 100 {
		if err := wal.Add([]byte("mehul"+fmt.Sprint(i)), []byte("kacky"), 2); err != nil {
			t.Fatalf("could not insert: %v", err)
		}
	}
	fmt.Println(time.Since(start))
}

func TestWalAddRecordConcurrent(t *testing.T) {
	now := time.Now()
	wal, _ := NewWal()
	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	doneChan := make(chan error, 1)
	iters := 9
	for i := range iters {
		wg.Add(1)
		go func(i int) {
			ch := doneChan
			err := wal.Add([]byte("mehul"+fmt.Sprint(i)), []byte("kacky"), 2)
			if err != nil {
				ch = errChan
			}
			wg.Done()
			ch <- err
		}(i)
	}
	wg.Wait()
	count := 0
LOOP:
	for {
		select {
		case <-doneChan:
			count++
			if count >= iters {
				break LOOP
			}
		case err := <-errChan:
			t.Fatalf("could not insert: %v", err)
		}
	}
	fmt.Printf("time taken for inserting %d records: %v\n", iters, time.Since(now))
}
