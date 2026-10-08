package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"
)

// RunFakeAgentIfRequested turns the current test binary into the fake agent when
// HILVAN_FAKE_AGENT=1. Call it first in TestMain; the binary then works as an agent CLI
// (os.Args[0]) on every OS, without Node.
func RunFakeAgentIfRequested() {
	if os.Getenv("HILVAN_FAKE_AGENT") == "1" {
		FakeAgent(os.Getenv("FAKE_SCENARIO"))
		os.Exit(0)
	}
}

// FakeAgentEnv returns env plus the variables that select the fake agent and its scenario.
func FakeAgentEnv(env map[string]string, scenario string) map[string]string {
	out := map[string]string{}
	for k, v := range env {
		out[k] = v
	}
	out["HILVAN_FAKE_AGENT"] = "1"
	out["FAKE_SCENARIO"] = scenario
	return out
}

func out(v any) {
	b, _ := json.Marshal(v)
	os.Stdout.Write(append(b, '\n'))
}

type o = map[string]any

// FakeAgent emits Claude-style stream-json for a scenario: success, write, review, limit,
// auth, crash, slow or huge (port of tests/fixtures/fake-agent.mjs).
func FakeAgent(scenario string) {
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
