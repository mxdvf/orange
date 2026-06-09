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

### Attempt 3 (~450 inserts/second)

My writes now use mmap I/O instead of traditional I/O which helps me skip the copying overhead of the user space buffer to kernel page cache. This was a milestone for me as converting a theorical concept into actual code and seeing it improve the performance (the way I expected it to) was quite an experience.

```
goos: darwin
goarch: arm64
pkg: github.com/mxdvf/btree/internal/btree
cpu: Apple M2
BenchmarkInsert
BenchmarkInsert-8            577           2422741 ns/op
BenchmarkInsert-8            631           2188987 ns/op
BenchmarkInsert-8            518           2290459 ns/op
BenchmarkInsert-8            607           1909189 ns/op
BenchmarkInsert-8            477           2183806 ns/op
PASS
ok      github.com/mxdvf/btree/internal/btree   6.698s
```
