package adapters

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/process"
)

// RunOptions bound a run.
type RunOptions struct {
	// Timeout is the hard limit for a run (SR-025); the run is cancelled when it expires.
	Timeout time.Duration
	// Grace is the time between SIGINT and SIGTERM, and between SIGTERM and SIGKILL (default 10 s).
	Grace             time.Duration
	MaxLine, MaxTotal int
	// OnLine is called once per stdout line, before its events (e.g. to store the run log).
	OnLine func(line string)
}

// RunResult is the outcome plus process facts.
type RunResult struct {
	Outcome
	ExitCode   int
	StderrTail string
}

// RunHandle controls a started run.
type RunHandle struct {
	PID    int
	cancel func(reason string)
	done   chan RunResult
}

// Cancel asks the agent to stop: SIGINT, then SIGTERM and SIGKILL after the grace period.
func (h *RunHandle) Cancel(reason string) { h.cancel(reason) }

// Wait blocks until the run ends.
func (h *RunHandle) Wait() RunResult { return <-h.done }

func sendSignal(p *os.Process, sig os.Signal) {
	if err := p.Signal(sig); err != nil && !errors.Is(err, os.ErrProcessDone) {
		p.Kill() // Windows cannot deliver SIGINT/SIGTERM to another process
	}
}

// StartRun spawns an agent CLI without a shell, writes the prompt to stdin, parses its JSONL
// stdout line by line with limits (SR-009) and decides the final state from the events.
// onEvent is called sequentially with each event and the raw line it came from.
func StartRun(inv launchers.Invocation, prompt string, parse func(string) []Event, onEvent func(Event, string), opts RunOptions) (*RunHandle, error) {
	grace := opts.Grace
	if grace == 0 {
		grace = 10 * time.Second
	}
	cmd, err := process.Command(context.Background(), inv.Command, inv.Args)
	h := &RunHandle{done: make(chan RunResult, 1), cancel: func(string) {}}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			h.done <- RunResult{Outcome: Outcome{State: "failed", Error: str(inv.Command + " not found")}, ExitCode: -1}
			return h, nil
		}
		return nil, err // unsafe arguments
	}
	if inv.Env != nil {
		cmd.Env = process.EnvList(inv.Env)
	}
	cmd.Dir = inv.Cwd
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		msg := err.Error()
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			msg = inv.Command + " not found"
		}
		h.done <- RunResult{Outcome: Outcome{State: "failed", Error: str(msg)}, ExitCode: -1}
		return h, nil
	}
	h.PID = cmd.Process.Pid

	var mu sync.Mutex
	cancelRequested := false
	limitError := ""
	var timers []*time.Timer
	h.cancel = func(reason string) {
		mu.Lock()
		defer mu.Unlock()
		if cancelRequested {
			return
		}
		cancelRequested = true
		if reason != "" && limitError == "" {
			limitError = reason
		}
		sendSignal(cmd.Process, os.Interrupt)
		timers = append(timers, time.AfterFunc(grace, func() {
			sendSignal(cmd.Process, syscall.SIGTERM)
			time.AfterFunc(grace, func() { cmd.Process.Kill() })
		}))
	}
	if opts.Timeout > 0 {
		mu.Lock()
		timers = append(timers, time.AfterFunc(opts.Timeout, func() { h.cancel("timeout") }))
		mu.Unlock()
	}

	go func() {
		io.WriteString(stdin, prompt) // the agent may exit before reading it; the outcome shows it
		stdin.Close()
	}()

	var events []Event
	splitter := process.NewLineSplitter(func(line string) {
		if opts.OnLine != nil {
			opts.OnLine(line)
		}
		for _, e := range parse(line) {
			events = append(events, e)
			onEvent(e, line)
		}
	})
	if opts.MaxLine > 0 {
		splitter.MaxLine = opts.MaxLine
	}
	if opts.MaxTotal > 0 {
		splitter.MaxTotal = opts.MaxTotal
	}

	var tail []byte
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]byte, 64*1024)
		for {
			n, err := stderr.Read(buf)
			tail = append(tail, buf[:n]...)
			if len(tail) > 4096 {
				tail = tail[len(tail)-4096:]
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		buf := make([]byte, 64*1024)
		stopped := false
		for {
			n, err := stdout.Read(buf)
			if n > 0 && !stopped {
				if perr := splitter.Push(string(buf[:n])); perr != nil {
					mu.Lock()
					limitError = fmt.Sprintf("output limit exceeded (%s)", perr.(*process.LineLimitError).Reason)
					mu.Unlock()
					stopped = true
					sendSignal(cmd.Process, syscall.SIGTERM)
				}
			}
			if err != nil {
				if !stopped {
					splitter.End()
				}
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		waitErr := cmd.Wait()
		mu.Lock()
		for _, t := range timers {
			t.Stop()
		}
		cancelled, limit := cancelRequested, limitError
		mu.Unlock()
		code := 0
		if waitErr != nil {
			code = -1
			var exit *exec.ExitError
			if errors.As(waitErr, &exit) {
				code = exit.ExitCode()
			}
		}
		stderrTail := process.DecodeOutput(tail)
		out := DecideOutcome(events, cancelled)
		if limit != "" && out.State == "uncertain" {
			out.Error = str(limit)
		}
		if limit == "timeout" {
			out.Error = str("timeout")
		}
		if (out.Error == nil || *out.Error == "") && out.State != "completed" && strings.TrimSpace(stderrTail) != "" {
			lines := strings.Split(strings.TrimSpace(stderrTail), "\n")
			out.Error = str(lines[len(lines)-1])
		}
		h.done <- RunResult{Outcome: out, ExitCode: code, StderrTail: stderrTail}
	}()
	return h, nil
}
