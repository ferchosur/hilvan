package process

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/ferchosur/hilvan/internal/security"
)

// Result is the outcome of a short-lived command.
type Result struct {
	// Code is the exit code, or -1 when the process could not start or was killed.
	Code   int
	Stdout string
	Stderr string
	// Err is "", "not_found", "timeout" or "spawn_failed".
	Err string
}

// Options for Run.
type Options struct {
	Timeout time.Duration // default 15 s
	// Env is the complete environment; nil inherits the current one.
	Env map[string]string
	Dir string
}

// EnvList turns an environment map into the sorted KEY=VALUE list exec expects.
func EnvList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// CurrentEnv returns this process's environment as a map.
func CurrentEnv() map[string]string {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
			env[k] = v
		}
	}
	return env
}

// Command builds an exec.Cmd without a shell. On Windows, batch files (.cmd/.bat, e.g. npm
// shims) are run through cmd.exe with every argument quoted and the metacharacters that quoting
// cannot neutralise rejected (ADR-0014, BatBadBut).
func Command(ctx context.Context, name string, args []string) (*exec.Cmd, error) {
	if err := security.AssertSafeArgs(args); err != nil {
		return nil, err
	}
	return command(ctx, name, args)
}

// Run executes a short-lived command and collects its output.
func Run(name string, args []string, opts Options) (Result, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd, err := Command(ctx, name, args)
	if err != nil {
		if isUnsafe(err) {
			return Result{}, err
		}
		return Result{Code: -1, Err: notFoundOr(err)}, nil
	}
	if opts.Env != nil {
		cmd.Env = EnvList(opts.Env)
	}
	cmd.Dir = opts.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	r := Result{Code: -1, Stdout: DecodeOutput(stdout.Bytes()), Stderr: DecodeOutput(stderr.Bytes())}
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		r.Err = "timeout"
	case err == nil:
		r.Code = 0
	default:
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			r.Code = exit.ExitCode()
		} else {
			r.Err = notFoundOr(err)
		}
	}
	return r, nil
}

// isUnsafe reports arguments refused for safety, which callers must see as errors.
func isUnsafe(err error) bool {
	return errors.Is(err, security.ErrUnsafeArgument) || errors.Is(err, ErrUnsafeBatchArgument)
}

func notFoundOr(err error) string {
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
		return "not_found"
	}
	return "spawn_failed"
}

// RunInteractive runs a command attached to the user's terminal (interactive login) and
// returns its exit code, or -1 if it could not start.
func RunInteractive(name string, args []string, env map[string]string, dir string) (int, error) {
	cmd, err := Command(context.Background(), name, args)
	if err != nil {
		if isUnsafe(err) {
			return -1, err
		}
		return -1, nil
	}
	if env != nil {
		cmd.Env = EnvList(env)
	}
	cmd.Dir = dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return -1, nil
	}
	return 0, nil
}

// DecodeOutput returns output as text. wsl.exe writes its own messages in UTF-16LE (spike 001),
// so output with a UTF-16LE BOM or mostly zero odd bytes is decoded as UTF-16.
func DecodeOutput(b []byte) string {
	if !looksUTF16LE(b) {
		return string(b)
	}
	if len(b) >= 2 && b[0] == 0xff && b[1] == 0xfe {
		b = b[2:]
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u))
}

func looksUTF16LE(b []byte) bool {
	if len(b) >= 2 && b[0] == 0xff && b[1] == 0xfe {
		return true
	}
	if len(b) < 4 || len(b)%2 != 0 {
		return false
	}
	zeros := 0
	for i := 1; i < len(b); i += 2 {
		if b[i] == 0 {
			zeros++
		}
	}
	return zeros*10 >= (len(b)/2)*8 // ≥ 80% of the high bytes are zero: ASCII text in UTF-16LE
}
