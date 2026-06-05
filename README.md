# BTree

A persistent copy-on-write B-Tree implementation, designed as an index for a key-value store.

## Design

We load raw bytes as a single page from the disk. When it's in the memory, we wrap it with Node to access methods to manipulate those bytes.

Disk (raw bytes) <--> Page Manager <--> Node (in-memory wrapper)

## Disk Layout (aka wire format)

```
|-------------------------------------- 4096 bytes -------------------------------------|
|   type    |   nkeys  |    pointers    |   offset-list  |       key-values             |
|    2B     |    2B    |   nkeys * 4B   |   nkeys * 2B	 |  [klen: 2B][k][vlen: 2B][v]  |
```

## Features

- [x] Persistent CoW B-tree: CoW enabling lock-free reads
- [x] fsync durability: ensure data survives OS crashes, not just process crashes
- [ ] Raw syscall I/O: mmap, fallocate, ftruncate, pwrite <!-- LOOK INTO THIS -->
- [ ] Free list management: reclaims pages from deleted or CoW-replaced nodes
- [ ] SIMD-accelerated comparisons: vectorized key search within nodes, better cache utilization
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

## License

MIT.
