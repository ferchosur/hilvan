//go:build !linux && !darwin

package main

import "os/exec"

// maxRSS is not measured on this OS; only the startup budget applies.
func maxRSS(*exec.Cmd) int64 { return 0 }
