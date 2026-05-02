### SuperfastKV

Calling it superfastkv would mean nothing if don't measure speed, latency, throughput and write speed and all that.
We'll also measure the actual speed and benchmark it with other in-memory stores like Redis, Memcache, especially Pogocache and also DiceDB.

Finish this with proper testing.

It's going to be fun. Every step of the journey could be an article:

- Building production-grade BTree implementation
- Benchmarking and testing your BTree implementation (because it's literally what powers the entire storage system)
- Benchmarking your kv store, comparing with other stores (can say you built something slower than all others but obviously i am still proud of it)
