//go:build !windows

package store

import (
	"errors"
	"syscall"
)

// ProcessAlive reports whether a process with this pid exists (signal 0 sends nothing).
func ProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
