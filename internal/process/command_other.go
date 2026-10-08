//go:build !windows

package process

import (
	"context"
	"os/exec"
)

func command(ctx context.Context, name string, args []string) (*exec.Cmd, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, err
	}
	return exec.CommandContext(ctx, path, args...), nil
}
