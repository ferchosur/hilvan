//go:build linux || darwin

package main

import (
	"os/exec"
	"runtime"
	"syscall"
)

// maxRSS is the child's peak resident set size in bytes.
func maxRSS(cmd *exec.Cmd) int64 {
	ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	if runtime.GOOS == "darwin" {
		return ru.Maxrss // bytes on macOS
	}
	return ru.Maxrss * 1024 // KiB on Linux
}
