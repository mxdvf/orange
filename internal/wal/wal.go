// Package wal provides the necessary tools to interact
// with the write-ahead-log for the main file
package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/mxdvf/orange/internal/btree"
)

type Wal struct {
	// wire format for wal records:
	// 	op  |   klen 	| 	key 	|  vlen	 |	value
	//  2B  |    2B  	|  [klen] |  	2B	 |  [vlen]
	file                   *os.File
	Cache                  sync.Map // the key for this map is: cacheEntry
	sync                   bool
	jobChan                chan commitRequest
	coordinationMu         sync.RWMutex
	checkpointMarkerOffset int64
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

func NewWal(sync bool) (*Wal, error) {
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
		sync:    sync,
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
		w.coordinationMu.Lock()
		// step 3: perform a single sequential write and a single fsync
		var err error
		if _, errWrite := w.file.Write(combinedBuf); errWrite != nil {
			err = fmt.Errorf("failed to append batch to wal: %w", errWrite)
		}
		if w.sync {
			if errSync := w.file.Sync(); errSync != nil {
				err = fmt.Errorf("failed to sync batch to disk: %w", errSync)
			}
		}
		w.coordinationMu.Unlock()
		// step 4: update memory cache and notify all waiting goroutines of the outcome
		for _, req := range batch {
			if err == nil {
				w.Cache.Store(req.key, req.entry)
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
			batch = append(batch, req)
			if len(batch) >= 10 {
				flush()
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flush()
			}
		}
	}
}

func (w *Wal) BackgroundJobLoop(tree *btree.BTree) {
	// TODO: the background job loop is absolutely bonkers right now
	// i have absolutely zero how it's even working or how is it even glued
	// together with the btree, i don't even know if it's working the way
	// it's supposed to work, like what on earth is it even doing, that's
	// so sad after writing this entire code.
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	// the job
	job := func(tree *btree.BTree) {
		offset := w.checkpointMarkerOffset
		root := tree.Root()
		count := 0
		for count < RecordEntriesThresholdNum {
			// read op code
			opBuf := make([]byte, OpSize)
			if _, err := w.file.ReadAt(opBuf, offset); err != nil || errors.Is(err, io.EOF) {
				return
			}
			op := binary.BigEndian.Uint16(opBuf)
			offset += OpSize
			// read klen
			klenBuf := make([]byte, KeyLenSize)
			if _, err := w.file.ReadAt(klenBuf, offset); err != nil {
				return
			}
			klen := binary.BigEndian.Uint16(klenBuf)
			offset += KeyLenSize
			// read key
			key := make([]byte, klen)
			if _, err := w.file.ReadAt(key, offset); err != nil {
				return
			}
			offset += int64(klen)
			// read vlen
			vlenBuf := make([]byte, ValLenSize)
			if _, err := w.file.ReadAt(vlenBuf, offset); err != nil {
				return
			}
			vlen := binary.BigEndian.Uint16(vlenBuf)
			offset += ValLenSize
			// read val
			value := make([]byte, vlen)
			if _, err := w.file.ReadAt(value, offset); err != nil {
				return
			}
			offset += int64(vlen)
			// perform the insertion here
			switch op {
			case InsertOp:
				// wire up the new root
				newRoot, err := tree.InsertNoSync(key, value, root)
				if err != nil {
					panic("insert failed")
				}
				root = newRoot
			case DeleteOp:
				if err := tree.Delete(key); err != nil {
					panic("delete failed")
				}
			case CheckpointOp:
				// Do nothing, this is just a structural log marker
			default:
				panic(fmt.Sprintf("unknown opcode encountered: %d", op))
			}
			// update the cache
			w.Cache.Delete(string(key))
			// also fsync the btree
			tree.Fsync()
		}
		// append a checkpoint to the wal
		w.coordinationMu.Lock()
		recordBuf := prepareRecord([]byte("#"), []byte("#"), CheckpointOp)
		if _, err := w.file.Write(recordBuf); err != nil {
			fmt.Println("failed to append batch to wal: %w", err)
			return
		}
		w.coordinationMu.Unlock()
		w.checkpointMarkerOffset = offset + int64(len(recordBuf))
	}
	// run every 10ms
	for range ticker.C {
		if w.getCacheLen() >= RecordEntriesThresholdNum {
			job(tree)
		}
	}
}

func (w *Wal) Get(k []byte) ([]byte, error) {
	e, ok := w.Cache.Load(string(k))
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
	w.Cache.Range(func(_, _ any) bool {
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
