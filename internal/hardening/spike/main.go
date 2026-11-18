//go:build linux

// Spike for pending item 5 (see docs/spikes/mlockall-memlock.md). Throwaway.
package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "lock" {
		fmt.Println("mlockall:", unix.Mlockall(unix.MCL_CURRENT|unix.MCL_FUTURE))
	}
	b := make([]byte, 64<<20)
	for i := range b {
		b[i] = 1
	}
	fmt.Println("allocated and touched 64 MiB")
}
