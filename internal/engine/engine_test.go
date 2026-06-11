package engine

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"sync"
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
	engine, err := NewEngine(filename, sync)
	if err != nil {
		t.Fatalf("cannot initialize engine: %v", err)
	}
	if t != nil {
		t.Logf("running test case for file: %v", filename)
	}
	return engine, filename
}

func TestBackgroundJob(t *testing.T) {
	eng, _ := setup(t, true)
	val := []byte("mehul")

	for i := range 300 {
		k := []byte("kacky-" + fmt.Sprint(i))
		if err := eng.Insert(k, val); err != nil && err != btree.ErrOverflow {
			t.Fatalf("insertion failed: %v", err)
		}
	}
	time.Sleep(10 * time.Second)
}

func TestBackgroundJobSearch(t *testing.T) {
	filename := "test/test-5749133887599376072.bin"
	engine, err := NewEngine(filename, true)
	if err != nil {
		t.Fatalf("cannot initialize engine: %v", err)
	}
	val := []byte("mehul")
	for i := range 300 {
		k := []byte("kacky-" + fmt.Sprint(i))
		retrievedVal, err := engine.btree.Search(k)
		if err != nil {
			t.Fatalf("search failed: %v", err)
		}
		if !bytes.Equal(val, retrievedVal) {
			t.Fatal("corrupt btree")
		}
	}
}

func BenchmarkInsertSingle(b *testing.B) {
	eng, _ := setup(nil, false)
	val := []byte("mehul")
	var i uint64

	b.ResetTimer()
	for b.Loop() {
		// Fast string concatenation + a single slice allocation
		k := []byte("kacky-" + strconv.FormatUint(i, 10))
		i++
		// NOTE this is eng.btree.Insert and not eng.Insert
		if err := eng.Insert(k, val); err != nil && err != btree.ErrOverflow {
			b.Fatalf("insertion failed: %v", err)
		}
	}
	b.StopTimer()
	totalInserts := float64(b.N)
	b.ReportMetric(totalInserts/b.Elapsed().Seconds(), "inserts/s")
}

func BenchmarkInsertConcurrent(b *testing.B) {
	eng, _ := setup(nil, true)
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
