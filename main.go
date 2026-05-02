package main

import (
	"fmt"
	"os"
	"time"
)

type Node struct {
	data []byte
}

func main() {
	f, _ := os.Create("wow.txt")
	now := time.Now()
	f.Write([]byte("wow"))
	// f.Sync()
	f.Close()
	fmt.Println("time taken: ", time.Since(now))
}
