# BTree

A persistent, crash-safe, copy-on-write B-Tree storage engine built from scratch in Go with zero dependencies. It currently hits:
- 1440 QPS for writes (batched)
- 560 QPS for writes (individual)
- 370,000 QPS for reads

> [!IMPORTANT]
> This is a project for learning purposes. It's extremely rough around the edges but that's the whole point of building it. After adding the WAL (even though experimental), the storage engine has become a little unstable. I am in the process of fixing that and a few other bugs following which, this could actually be considered a production grade storage engine.

## Disk Layout (aka wire format)

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
- [x] Experimental WAL\*\*: append-only recovery log with group commits and WAL-first lookups

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
