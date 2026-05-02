Tips on how to make the implementation faster:

- Replace linear scan with: binary search (large node fanout) or SIMD scan (if you go hardcore)
- Keep node size aligned to cache lines (typically 64B / 128B)
- Store keys in a contiguous array (you likely already do)
- Consider branchless comparisons

Tradeoffs

- use variable keys and values (not all kv pairs are same -- fixed length keys and values in the btree)
- use
- implement CoW semantics (look into stratified btree - https://arxiv.org/pdf/1103.4282v2)
- using 2 bytes for pointers, restricting database size to 256MB. let's see what breaks.
