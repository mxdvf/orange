### Philosophy behind the code

Pretty simple. We load raw bytes as a single page from the disk, it's in the memory, we wrap it with NewNode to access methods to manipulate those bytes
Disk (raw bytes) <--> Page (in memory) <--> Node (wrapper)

In code, btree.go contains the code to orchestrate the crud on btree with CoW semantics. It uses page_manager.go to allocate/write/read pages and node.go to instantiate a wrapper on the raw bytes.
