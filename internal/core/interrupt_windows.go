//go:build windows

package core

import "os"

// Windows cannot deliver SIGINT to another process; the run is stopped and the task ends
// uncertain (no final event), never silently re-run.
func interrupt(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
