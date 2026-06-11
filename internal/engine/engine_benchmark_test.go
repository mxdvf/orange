package engine

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mxdvf/orange/internal/btree"
)

func init() {
	err := os.MkdirAll("test/", 0755)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func setup(t testing.TB, sync bool) (*Engine, string) {
	filename := fmt.Sprintf("test/test-%v.bin", rand.Int())
	if t != nil {
		t.Logf("running test case for file: %v", filename)
	}
	engine, err := NewEngine(filename, sync)
	if err != nil {
		t.Fatalf("cannot initialize engine: %v", err)
	}

	return engine, filename
}

func TestRawThroughputInsertParallel(t *testing.T) {
	engine, _ := setup(t, true)
	val := []byte("mehul")
	const n = 10000
	var wg sync.WaitGroup
	var counter atomic.Uint64

	start := time.Now()
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			i := counter.Add(1)
			k := []byte("kacky-" + strconv.FormatUint(i, 10))
			engine.Insert(k, val)
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("total: %v", elapsed)
	t.Logf("per insert: %v", elapsed/time.Duration(n))
	t.Logf("inserts/sec: %.0f", float64(n)/elapsed.Seconds())
}

// benchmarks

func BenchmarkInsert(b *testing.B) {
	tr, _ := setup(nil, false)
	val := []byte("mehul")
	var i uint64

	b.ResetTimer()
	for b.Loop() {
		// Fast string concatenation + a single slice allocation
		k := []byte("kacky-" + strconv.FormatUint(i, 10))
		i++
		// NOTE this is eng.btree.Insert and not eng.Insert
		if err := tr.btree.Insert(k, val); err != nil && err != btree.ErrOverflow {
			b.Fatalf("insertion failed: %v", err)
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	// pre-populate the tree with 100k keys before benchmarking with fsync switched off
	eng, _ := setup(nil, false)
	val := []byte("mehul")
	const numKeys = 100_000
	for i := range uint64(numKeys) {
		k := []byte("kacky-" + strconv.FormatUint(i, 10))
		// NOTE this is eng.btree.Insert and not eng.Insert
		if err := eng.btree.Insert(k, val); err != nil && err != btree.ErrOverflow {
			b.Fatalf("setup insertion failed: %v", err)
		}
	}

	time.Sleep(2 * time.Second)

	var i uint64
	// reset timer so setup cost is excluded from benchmark
	b.ResetTimer()
	for b.Loop() {
		// scatter across the full key space — not sequential
		// this forces the btree to traverse different paths each time
		idx := (i * 6364136223846793005) % numKeys
		k := []byte("kacky-" + strconv.FormatUint(idx, 10))
		i++
		if _, err := eng.btree.Search(k); err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}
