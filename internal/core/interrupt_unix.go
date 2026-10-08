//go:build !windows

package core

import "syscall"

func interrupt(pid int) error { return syscall.Kill(pid, syscall.SIGINT) }
