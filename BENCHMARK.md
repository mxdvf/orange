## Benchmark

### Attempt 1 (~130 inserts/second)

At this point my insert has to finish 2 fsync() calls every single time.

```
goos: darwin
goarch: arm64
pkg: github.com/mxdvf/btree/internal/btree
cpu: Apple M2
BenchmarkInsert
BenchmarkInsert-8            145           8366655 ns/op
BenchmarkInsert-8            158           7501669 ns/op
BenchmarkInsert-8            162           7576352 ns/op
BenchmarkInsert-8            153           7540920 ns/op
BenchmarkInsert-8            159           7447241 ns/op
PASS
ok      github.com/mxdvf/btree/internal/btree     6.942s
```

### Attempt 2 (~160 inserts/second)

Changed my page allocator to now allocate large chunks upfront to speed up subsequent writes. Used fallocate() for Linux and fcntlFstore() for Darwin. Not much improvement contrary to what I was expecting but I shouldn't have because my 2-fsync barrier is clearly a major bottleneck.

```
goos: darwin
goarch: arm64
pkg: github.com/mxdvf/btree/internal/btree
cpu: Apple M2
BenchmarkInsert
BenchmarkInsert-8            166           6937548 ns/op
BenchmarkInsert-8            182           6444283 ns/op
BenchmarkInsert-8            162           6533503 ns/op
BenchmarkInsert-8            205           6126638 ns/op
BenchmarkInsert-8            210           6194753 ns/op
PASS
ok      github.com/mxdvf/btree/internal/btree   6.203s
```
