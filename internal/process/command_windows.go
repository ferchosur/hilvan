//go:build windows

package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func command(ctx context.Context, name string, args []string) (*exec.Cmd, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".cmd" && ext != ".bat" {
		return exec.CommandContext(ctx, path, args...), nil
	}
	line, err := BatchCommandLine(path, args)
	if err != nil {
		return nil, err
	}
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
	}
	cmd := exec.CommandContext(ctx, comspec)
	// The command line starts with the program itself, as CreateProcess expects.
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + comspec + `" ` + line}
	return cmd, nil
}
