package engine

import (
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mxdvf/orange/btree"
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
	engine, err := NewEngine(filename, sync)
	if err != nil {
		t.Fatalf("cannot initialize engine: %v", err)
	}
	if t != nil {
		t.Logf("running test case for file: %v", filename)
	}
	return engine, filename
}

func TestInsertSingle(t *testing.T) {
	eng, _ := setup(nil, true)
	k := []byte("kacky-0")
	val := []byte("mehul")
	now := time.Now()
	if err := eng.Insert(k, val); err != nil && err != btree.ErrOverflow {
		t.Fatalf("insertion failed: %v", err)
	}
	fmt.Println(time.Since(now))
}

func TestInsertMultipleSequentialDirectBTree(t *testing.T) {
	eng, _ := setup(nil, false)
	val := []byte("mehul")
	var wg sync.WaitGroup
	for i := range 40_000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := eng.Insert([]byte("kacky-"+strconv.Itoa(i)), val); err != nil && err != btree.ErrOverflow {
				panic("failed insertion")
			}
			fmt.Println("done", i)
		}()
	}
	wg.Wait()
}

func BenchmarkInsertSingle(b *testing.B) {
	eng, _ := setup(b, true)
	val := []byte("mehul")
	var i uint64

	b.ResetTimer()
	for b.Loop() {
		// fast string concatenation + a single slice allocation
		k := []byte("kacky-" + strconv.FormatUint(i, 10))
		fmt.Println(k)
		i++
		now := time.Now()
		if err := eng.Insert(k, val); err != nil && err != btree.ErrOverflow {
			b.Fatalf("insertion failed: %v", err)
		}
		fmt.Println("time taken: ", time.Since(now))
	}
	b.StopTimer()
	totalInserts := float64(b.N)
	b.ReportMetric(float64(b.N), "keys")
	b.ReportMetric(totalInserts/b.Elapsed().Seconds(), "inserts/s")
}

func BenchmarkInsertConcurrent(b *testing.B) {
	eng, filename := setup(b, true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		var firstErr error
		var errOnce sync.Once
		iters := 1000
		for j := 0; j < iters; j++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				err := eng.Insert([]byte("mehul"+fmt.Sprint(id)), []byte("kacky"))
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
					})
				}
			}(j)
		}
		wg.Wait()
		if firstErr != nil {
			b.Fatalf("could not insert: %v", firstErr)
		}
	}
	b.StopTimer()
	totalInserts := float64(b.N * 1000)
	info, _ := os.Stat(filename)
	b.Logf("b.N=%d file size=%.1fMB", b.N, float64(info.Size())/1e6)
	b.ReportMetric(totalInserts/b.Elapsed().Seconds(), "inserts/s")
}

func BenchmarkSearch(b *testing.B) {
	eng, _ := setup(nil, false)
	val := []byte("mehul")
	const numKeys = 100_000
	keys := make([][]byte, numKeys)
	for i := range uint64(numKeys) {
		keys[i] = []byte("kacky-" + strconv.FormatUint(i, 10))

		if err := eng.Insert(keys[i], val); err != nil && err != btree.ErrOverflow {
			b.Fatalf("setup insertion failed: %v", err)
		}
	}
	var i uint64
	b.ResetTimer()
	for b.Loop() {
		idx := (i * 6364136223846793005) % numKeys
		i++
		if _, err := eng.Search(keys[idx]); err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
	b.StopTimer()
	totalSearches := float64(b.N)
	b.ReportMetric(totalSearches/b.Elapsed().Seconds(), "searches/s")
}
