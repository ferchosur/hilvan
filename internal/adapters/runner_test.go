package adapters

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/process"
)

// The test binary doubles as a fake agent CLI emitting Claude-style stream-json, driven by
// FAKE_SCENARIO (port of tests/fixtures/fake-agent.mjs).
func TestMain(m *testing.M) {
	if os.Getenv("HILVAN_FAKE_AGENT") == "1" {
		fakeAgent(os.Getenv("FAKE_SCENARIO"))
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func out(v any) {
	b, _ := json.Marshal(v)
	os.Stdout.Write(append(b, '\n'))
}

type o = map[string]any

func fakeAgent(scenario string) {
	if scenario == "" {
		scenario = "success"
	}
	var sigint chan os.Signal
	if scenario == "slow" {
		sigint = make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
	}
	prompt, _ := io.ReadAll(os.Stdin)
	out(o{"type": "system", "subtype": "init", "session_id": "sess-1", "model": "fake-1", "mcp_servers": []any{}})
	out(o{"type": "assistant", "message": o{"content": []any{o{"type": "text", "text": fmt.Sprintf("got %d chars \x1b]52;c;ZXZpbA==\x07", len(prompt))}}}})
	if scenario == "write" || scenario == "success" {
		out(o{"type": "assistant", "message": o{"content": []any{o{"type": "tool_use", "name": "Edit", "input": o{"file_path": "hello.txt"}}}}})
		if scenario == "write" {
			os.WriteFile("hello.txt", []byte("hello from the agent\n"), 0o644)
		}
	}
	switch scenario {
	case "limit":
		out(o{"type": "system", "subtype": "api_retry", "attempt": 1, "error": "rate_limit"})
		out(o{"type": "rate_limit_event", "rate_limit_info": o{"status": "rejected", "unifiedWindows": o{"five_hour": o{"utilization": 1, "resetsAt": 1}}}})
		out(o{"type": "result", "subtype": "error_during_execution", "is_error": true, "api_error_status": 429, "session_id": "sess-1"})
		return
	case "auth":
		out(o{"type": "result", "subtype": "error_during_execution", "is_error": true, "api_error_status": 401, "session_id": "sess-1"})
		return
	case "crash":
		os.Exit(3)
	case "huge":
		os.Stdout.WriteString(`{"type":"assistant","x":"` + strings.Repeat("a", 3*1024*1024) + "\"}\n")
	case "slow":
		select {
		case <-sigint:
			out(o{"type": "result", "subtype": "error_during_execution", "is_error": true, "session_id": "sess-1"})
			os.Exit(0)
		case <-time.After(30 * time.Second):
		}
	}
	out(o{"type": "rate_limit_event", "rate_limit_info": o{"status": "allowed", "unifiedWindows": o{"five_hour": o{"utilization": 0.4, "resetsAt": 1}, "seven_day": o{"utilization": 0.2, "resetsAt": 2}}}})
	result := "Done. token sk-ant-api03-AAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if scenario == "review" {
		result = "- high: missing test for hello.txt\nVERDICT: changes_requested"
	}
	out(o{"type": "result", "subtype": "success", "is_error": false, "result": result, "session_id": "sess-1",
		"total_cost_usd": 0.0123, "usage": o{"input_tokens": 10, "output_tokens": 5}, "permission_denials": []any{}})
}

// FakeInvocation runs the fake agent with a scenario.
func FakeInvocation(scenario string) launchers.Invocation {
	env := process.CurrentEnv()
	env["HILVAN_FAKE_AGENT"] = "1"
	env["FAKE_SCENARIO"] = scenario
	return launchers.Invocation{Command: os.Args[0], Env: env}
}

func run(t *testing.T, scenario string, opts RunOptions) (*RunHandle, *[]Event) {
	t.Helper()
	events := &[]Event{}
	h, err := StartRun(FakeInvocation(scenario), "hello prompt", ParseClaudeLine, func(e Event, _ string) { *events = append(*events, e) }, opts)
	if err != nil {
		t.Fatal(err)
	}
	return h, events
}

func TestRunnerCompletes(t *testing.T) {
	h, events := run(t, "success", RunOptions{})
	r := h.Wait()
	if r.State != "completed" || r.CostUSD == nil || *r.CostUSD != 0.0123 {
		t.Fatalf("outcome %+v", r.Map())
	}
	found := false
	for _, e := range *events {
		found = found || (e.Type == EvMessage && strings.HasPrefix(e.Text, "got 12 chars"))
	}
	if !found {
		t.Error("the prompt did not reach the agent through stdin")
	}
}

func TestRunnerBlocks(t *testing.T) {
	for scenario, reason := range map[string]string{"limit": "rate_limit", "auth": "auth"} {
		h, _ := run(t, scenario, RunOptions{})
		if r := h.Wait(); r.State != "blocked" || r.BlockedReason != reason {
			t.Errorf("%s: %+v", scenario, r.Map())
		}
	}
}

func TestRunnerCrashIsUncertain(t *testing.T) {
	h, _ := run(t, "crash", RunOptions{})
	if r := h.Wait(); r.State != "uncertain" || r.ExitCode != 3 {
		t.Errorf("crash: %+v exit %d", r.Map(), r.ExitCode)
	}
}

func TestRunnerCancel(t *testing.T) {
	h, _ := run(t, "slow", RunOptions{Grace: 2 * time.Second})
	time.AfterFunc(300*time.Millisecond, func() { h.Cancel("") })
	if r := h.Wait(); r.State != "cancelled" {
		t.Errorf("cancel: %+v", r.Map())
	}
}

func TestRunnerOutputLimit(t *testing.T) {
	h, _ := run(t, "huge", RunOptions{MaxLine: 1024 * 1024})
	r := h.Wait()
	if r.State != "uncertain" || r.Error == nil || !strings.Contains(*r.Error, "output limit") {
		t.Errorf("SR-009: %+v", r.Map())
	}
}

func TestRunnerTimeout(t *testing.T) {
	h, _ := run(t, "slow", RunOptions{Timeout: 300 * time.Millisecond, Grace: 200 * time.Millisecond})
	r := h.Wait()
	if r.State != "cancelled" || r.Error == nil || *r.Error != "timeout" {
		t.Errorf("SR-025: %+v", r.Map())
	}
}

func TestRunnerMissingExecutable(t *testing.T) {
	h, err := StartRun(launchers.Invocation{Command: filepath.Join(t.TempDir(), "agent")}, "", ParseClaudeLine, func(Event, string) {}, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r := h.Wait(); r.State != "failed" {
		t.Errorf("missing executable: %+v", r.Map())
	}
}

func TestRunnerRejectsUnsafeArguments(t *testing.T) {
	inv := FakeInvocation("success")
	inv.Args = []string{"a\nb"}
	if _, err := StartRun(inv, "", ParseClaudeLine, func(Event, string) {}, RunOptions{}); err == nil {
		t.Error("newline argument accepted")
	}
}
