package pagemanager

import (
	"fmt"
	"os"
)

const (
	MockPageSize = 4096
	MockSync     = false
)

func init() {
	err := os.MkdirAll("test/", 0755)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// TODO: testing my page manager has become quite complex, setup a better test infra
// it's literally allocating space on disk, and how do i even test mmap, i think same
// is needed for the btree also
