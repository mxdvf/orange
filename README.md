# BTree

A persistent, crash-safe, copy-on-write B-Tree storage engine built from scratch in Go with zero dependencies.

## Overview

We load raw bytes as a single page from the disk. When it's in the memory, we wrap it with Node to access methods to manipulate those bytes.

Disk (raw bytes) <--> Page Manager <--> Node (in-memory wrapper)

## Disk Layout (aka wire format)

```
|-------------------------------------- 4096 bytes -------------------------------------|
|   type    |   nkeys  |    pointers    |   offset-list  |       key-values             |
|    2B     |    2B    |   nkeys * 4B   |   nkeys * 2B	 |  [klen: 2B][k][vlen: 2B][v]  |
```

## Features

- [x] Persistent CoW B-tree: immutable pages enabling lock-free reads
- [x] fsync durability: ensure data survives OS crashes, not just process crashes
- [x] Page allocator: fallocate/fcntl to pre-allocate pages (requires conditional build for macos/linux)
- [x] Memory-mapped I/O: replace traditional I/O with memory-mapping using mmap() and mremap()
- [x] Free list management: track and reclaim pages from deleted or CoW-replaced nodes
- [ ] WAL: append-only log with LSN, group commit and crash recovery with CRC32 validation
- [ ] Benchmarking: evaluates r/w latency and throughput under concurrent workloads

## API

```go
// Write some data.
err := btree.Insert([]byte("key"), []byte("val"))
btree.Insert([]byte("key1"), []byte("val1"))
btree.Insert([]byte("key2"), []byte("val2"))

// Read it back.
v, err := btree.Search([]byte("key1")) // val1, nil
btree.Search([]byte("key90")) // nil, ErrKeyNotFound

// Delete some data.
err := btree.Delete([]byte("key1")) // nil
```

## Special Mentions

- https://blog.minhazav.dev/memory-sharing-in-linux/#misc-mmap-is-faster-than-reading-a-file-in-blocks
- https://transactional.blog/blog/2025-torn-writes
- https://www.youtube.com/watch?v=OtxCzIHOMk4
- https://www.usenix.org/system/files/fast25-jeon.pdf
- A lot of discussions with Claude (Anthropic) because I don't know what I don't know

## License

MIT.
