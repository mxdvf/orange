### SuperfastKV

Calling it superfastkv would mean nothing if don't measure speed, latency, throughput and write speed and all that.
We'll also measure the actual speed and benchmark it with other in-memory stores like Redis, Memcache, especially Pogocache and also DiceDB.

Finish this with proper testing.

It's going to be fun. Every step of the journey could be an article:

- Building production-grade BTree implementation
- Benchmarking and testing your BTree implementation (because it's literally what powers the entire storage system)
- Benchmarking your kv store, comparing with other stores (can say you built something slower than all others but obviously i am still proud of it)

### BTree

Tips on how to make the implementation faster:

- Replace linear scan with: binary search (large node fanout) or SIMD scan (if you go hardcore)
- Keep node size aligned to cache lines (typically 64B / 128B)
- Store keys in a contiguous array (you likely already do)
- Consider branchless comparisons

Tradeoffs

- use variable keys and values (not all kv pairs are same -- fixed length keys and values in the btree)
- implement CoW semantics (or look into stratified btree - https://arxiv.org/pdf/1103.4282v2 to make it faster)
- using 2 bytes for pointers, restricting database size to 256MB. let's see what breaks.
