# BTree

A production-grade, persistent and crash-safe CoW B-Tree storage engine built from scratch in Go with zero dependencies. It can hit:

- Write throughput: 32K writes/sec
- Read throughput: 370K reads/sec

> [!IMPORTANT]
> This is a project built for learning purposes. It is not intended for use in production.

## Disk Layout

```
|------------------------------------ 4096 bytes ---------------------------------------|
|   type    |   nkeys  |    pointers    |   offset-list  |       key-values             |
|    2B     |    2B    |   nkeys * 4B   |   nkeys * 2B	 |  [klen: 2B][k][vlen: 2B][v]  |
```

## Features

- [x] Persistent CoW B-tree: immutable pages enabling lock-free reads
- [x] fsync durability: ensure data survives OS crashes, not just process crashes
- [x] Page allocator: fallocate/fcntl to pre-allocate pages (requires conditional build for macos/linux)
- [x] Memory-mapped I/O: replace traditional I/O with memory-mapping using mmap() and mremap()
- [x] Free list management: track and reclaim pages from deleted or CoW-replaced nodes
- [x] Group commit: engine now uses batching to commit writes in groups

## API

```go
// Write some data.
err := engine.Insert([]byte("key"), []byte("val"))
engine.Insert([]byte("key1"), []byte("val1"))
engine.Insert([]byte("key2"), []byte("val2"))

// Read it back.
v, err := engine.Search([]byte("key1")) // val1, nil
engine.Search([]byte("key90")) // nil, ErrKeyNotFound

// Delete some data.
err := engine.Delete([]byte("key1")) // nil
```

## Special Mentions

- https://blog.minhazav.dev/memory-sharing-in-linux/#misc-mmap-is-faster-than-reading-a-file-in-blocks
- https://transactional.blog/blog/2025-torn-writes
- https://www.usenix.org/system/files/fast25-jeon.pdf
- A lot of discussions with Claude (Anthropic) because I don't know what I don't know
- https://www.youtube.com/watch?v=OtxCzIHOMk4
- https://www.scylladb.com/2017/10/05/io-access-methods-scylla/

## License

MIT.
