// Package wal provides the necessary tools to interact
// with the write-ahead-log for the main file
package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type Wal struct {
	// wire format for wal records:
	// 	op  |   klen 	| 	key 	|  vlen	 |	value
	//  2B  |    2B  	|  [klen] |  	2B	 |  [vlen]
	file                   *os.File
	cache                  sync.Map // the key for this map is: cacheEntry
	jobChan                chan commitRequest
	coordinationMu         sync.RWMutex
	checkpointMarkerOffset int
}

type commitRequest struct {
	buf     []byte
	key     string
	entry   cacheEntry
	errChan chan error // channel to send the sync result back to the caller
}

type cacheEntry struct {
	op    uint16
	value []byte
}

const (
	RecordEntriesThresholdNum = 200

	InsertOp     uint16 = iota // 0
	DeleteOp                   // 1
	CheckpointOp               // 2
)

const (
	OpSize     = 2 // 2  bytes
	KeyLenSize = 2 // 2 bytes
	ValLenSize = 2 // 2 bytes
)

var (
	ErrKeyDoesNotExist = errors.New("failed to find the key in wal, requires traversal")
	ErrKeyDeleted      = errors.New("key has been deleted")
)

func NewWal() (*Wal, error) {
	// TODO: the wal file must also not grow unbounded, i know this
	// is a very fundamental feature but i just don't have the mental
	// bandwidth to work on this anymore. if at all, you start working
	// on this project, i think we must first fix this issue
	// setup the wal file
	file, err := setupFile()
	if err != nil {
		return nil, fmt.Errorf("wal initialization failed: %w", err)
	}
	w := &Wal{
		file:    file,
		jobChan: make(chan commitRequest, 1000),
	}
	go w.groupCommitLoop()
	// TODO: if the machine crashes, this is the point where you
	// perform a sophisticated crash recovery, skipping checkpoints
	// and collecting all writes and then flushing them to the main
	// db (aka btree). it should happen as if you're starting from a
	// clean slate, the startup can take time and it is meant to take
	// time so that's completely alright.
	// return the wal
	return w, nil
}

func (w *Wal) Add(k, v []byte, op uint16) error {
	// setup the record buffer to be appended
	recordBuf := prepareRecord(k, v, op)
	// setup an error channel which helps in blocking
	// clients that called this method
	errChan := make(chan error, 1)
	// setup the request
	req := commitRequest{
		buf:     recordBuf,
		key:     string(k),
		entry:   cacheEntry{op, v},
		errChan: errChan,
	}
	// send this request on the job channel, there's
	// a possibility, this might block because the job
	// channel can only ever handle 1000 entries and that
	// too only process 10 at a time
	w.jobChan <- req
	// in case someone takes out the request from the
	// jobChan, this is how we keep the client engaged
	// without responding, until get back some result
	err := <-errChan
	return err
}

func (w *Wal) groupCommitLoop() {
	// acquire locks for coordination, particularly
	// so that a backround job just does not start
	// till the group commit loop has the lock acquired
	w.coordinationMu.Lock()
	defer w.coordinationMu.Unlock()
	// setup
	batch := make([]commitRequest, 0, 10)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	// flush is a helper function to execute the batch
	// write and perform a batched-sync too, this is
	// a naive way of boosting our throughput
	flush := func() {
		if len(batch) == 0 {
			return
		}
		// step 1: pre-allocate buffer to avoid dynamic slice resizing allocations
		var totalLen int
		for _, req := range batch {
			totalLen += len(req.buf)
		}
		combinedBuf := make([]byte, 0, totalLen)
		// step 2: merge all separate records into a single continuous byte array
		for _, req := range batch {
			combinedBuf = append(combinedBuf, req.buf...)
		}
		// step 3: perform a single sequential write and a single fsync
		var err error
		if _, err = w.file.Write(combinedBuf); err != nil {
			err = fmt.Errorf("failed to append batch to wal: %w", err)
		} else if err = w.file.Sync(); err != nil {
			err = fmt.Errorf("failed to sync batch to disk: %w", err)
		}
		// step 4: update memory cache and notify all waiting goroutines of the outcome
		for _, req := range batch {
			if err == nil {
				w.cache.Store(req.key, req.entry)
			}
			req.errChan <- err
		}
		// step 5: reset batch storage for the next cycle
		batch = batch[:0]
		// reset ticker to ensure the next batch gets a full 10ms window
		ticker.Reset(10 * time.Millisecond)
	}

	for {
		select {
		case req := <-w.jobChan:
			fmt.Println("is this called? 1")
			batch = append(batch, req)
			if len(batch) >= 10 {
				flush()
			}
		case <-ticker.C:
			fmt.Println("is this called? 2")
			if len(batch) > 0 {
				flush()
			}
			if w.getCacheLen() > RecordEntriesThresholdNum {
				// go w.backgroundJobLoop()
			}
		}
	}
}

func (w *Wal) Get(k []byte) ([]byte, error) {
	e, ok := w.cache.Load(string(k))
	if !ok {
		return nil, ErrKeyDoesNotExist
	}
	entry := e.(cacheEntry)
	if entry.op == DeleteOp {
		return nil, ErrKeyDeleted
	}
	return entry.value, nil
}

func (w *Wal) getCacheLen() int {
	length := 0
	w.cache.Range(func(_, _ any) bool {
		length++
		return true
	})
	return length
}

func prepareRecord(k, v []byte, op uint16) []byte {
	// create a buffer of specific length
	totalLen := OpSize + KeyLenSize + len(k) + ValLenSize + len(v)
	buf := make([]byte, totalLen)
	// start wrtiting to the buf with seek technique
	start := 0
	binary.BigEndian.PutUint16(buf[start:], op)
	start += OpSize
	binary.BigEndian.PutUint16(buf[start:], uint16(len(k)))
	start += KeyLenSize
	copy(buf[start:], k)
	start += len(k)
	binary.BigEndian.PutUint16(buf[start:], uint16(len(v)))
	start += ValLenSize
	copy(buf[start:], v)
	return buf
}
