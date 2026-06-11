//go:build darwin

package wal

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

func setupFile() (*os.File, error) {
	// setup name for the file
	name := "wal_" + strconv.FormatInt(rand.Int63(), 10) + ".wal"
	// create the wal
	file, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to create the wal: %w", err)
	}
	// set F_NOCACHE on the fd to disable OS caching (equivalent of)
	// initialize file as O_DIRECT mode
	fd := int(file.Fd())
	_, _, errno := syscall.Syscall(
		syscall.SYS_FCNTL,
		uintptr(fd),
		uintptr(unix.F_NOCACHE),
		1, // 1 turns caching OFF, 0 turns it ON
	)
	if errno != 0 {
		return nil, fmt.Errorf("failed to set the F_NOCACHE mode: %w", err)
	}
	// return the fd
	return file, nil
}
