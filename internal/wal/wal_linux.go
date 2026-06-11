//go:build linux

package wal

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"golang.org/x/sys/unix"
)

func setupFile() (*os.File, error) {
	// setup name for the file
	name := "wal_" + strconv.FormatInt(rand.Int63(), 10) + ".wal"
	// create the wal
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_RDWR|unix.O_DIRECT, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open the wal file: %w", err)
	}
	// persist the file
	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("failed to fsync the file: %w", err)
	}
	// return the fd
	return file, nil
}
