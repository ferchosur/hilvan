package adapters

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/process"
)

// RunOnce runs a one-shot command: the text goes to stdin once, and stdout is returned. It never
// uses a shell, and it is killed when the timeout expires (used by the chat interpreter).
func RunOnce(inv launchers.Invocation, stdin string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd, err := process.Command(ctx, inv.Command, inv.Args)
	if err != nil {
		return "", err
	}
	if inv.Env != nil {
		cmd.Env = process.EnvList(inv.Env)
	}
	cmd.Dir = inv.Cwd
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errors.New("the interpreter did not answer in time")
		}
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), errors.New("the interpreter failed: " + msg)
	}
	return out.String(), nil
}
