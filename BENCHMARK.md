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

### Attempt 4 (~560 inserts/second)

I could squeeze in ~24% extra throughput by using the concept of free(-page-)list. Which basically means re-using the abandoned/deleted pages because of CoW semantics. But another major win it gets me is by not continously allocating more and more physical space on the disk because I can just re-use whatever I allocated myself previously.

Biggest win came from the fact that my engine's disk usage has gone down by ~92%. That's massive. Earlier one of my test cases was churning out 547MB files, not it does only 41MB.

```
goos: darwin
goarch: arm64
pkg: github.com/mxdvf/orange/internal/btree
cpu: Apple M2
BenchmarkInsert
BenchmarkInsert-8            691           1751633 ns/op
BenchmarkInsert-8            699           1806480 ns/op
BenchmarkInsert-8            682           1743364 ns/op
BenchmarkInsert-8            628           1885124 ns/op
BenchmarkInsert-8            690           1771671 ns/op
PASS
ok      github.com/mxdvf/orange/internal/btree  6.593s
```

### Attempt 5 (~1440 inserts/second with batching)

My WAL implementation that makes use of group commits helped me achieve this number. But sadly, it's in such experimental stage right now that talking about it would be me overselling it. But I discovered an important insight, this is technically a never-ending project. Putting a deadline on this is nearly impossible, as soon as I thought of implementing WAL, there were thousands of different things that I had to keep in mind. And the way I implemented WAL feels like the feature itself is fighting against me, not with me. So that's the point I knew I had to stop and take a break.

```
goos: darwin
goarch: arm64
pkg: github.com/mxdvf/orange/internal/engine
cpu: Apple M2
BenchmarkInsertConcurrent
BenchmarkInsertConcurrent-8            2         576153896 ns/op              1736 inserts/s
BenchmarkInsertConcurrent-8            2         629381312 ns/op              1589 inserts/s
BenchmarkInsertConcurrent-8            2         704476834 ns/op              1419 inserts/s
BenchmarkInsertConcurrent-8            2         741241854 ns/op              1349 inserts/s
BenchmarkInsertConcurrent-8            2         904052833 ns/op              1106 inserts/s
PASS
ok      github.com/mxdvf/orange/internal/engine 10.379s
```

### Attempt 6 (~32746 inserts/second)

So I finally got rid of the entire WAL implemnentation. It just kept adding unnecessary complexity and I wouldn't deny it was a hasty attempt at getting my write throughput above 1K inserts/second. Once I got that thought of my head, it was just me and my first principles thinking. The simplest thing I could do was perform group commits on top of my already existing BTree implementation. And that's exactly what I did. And it got me the results I exactly hoped for. Along with the safety guarantees.

```
goos: darwin
goarch: arm64
pkg: github.com/mxdvf/orange/engine
cpu: Apple M2
BenchmarkInsertConcurrent-8           44          29930696 ns/op             33411 inserts/s
BenchmarkInsertConcurrent-8           73          30739960 ns/op             32531 inserts/s
BenchmarkInsertConcurrent-8           51          30254415 ns/op             33053 inserts/s
BenchmarkInsertConcurrent-8           75          30605274 ns/op             32674 inserts/s
BenchmarkInsertConcurrent-8           78          31188058 ns/op             32064 inserts/s
PASS
ok      github.com/mxdvf/orange/engine  10.211s
```
