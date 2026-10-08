package adapters

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/testutil"
)

// Golden cases from the TypeScript adapters (scripts/parity/gen-g2.ts, retired in G5: see commit 95e1d04).
type adaptersGolden struct {
	Claude []struct {
		Line   string `json:"line"`
		Events []any  `json:"events"`
	} `json:"claude"`
	Codex [][]struct {
		Line   string `json:"line"`
		Events []any  `json:"events"`
	} `json:"codex"`
	Outcomes []struct {
		Events []map[string]any `json:"events"`
		Cancel bool             `json:"cancel"`
		Out    any              `json:"out"`
	} `json:"outcomes"`
	Auth []struct {
		App, Stdout, Stderr string
		Code                int
		Status              any
	} `json:"auth"`
	Tools []struct {
		App, Stdout string
		Tools       any
	} `json:"tools"`
	Commands []struct {
		Profile domain.Profile `json:"profile"`
		Env     any            `json:"env"`
		Login   any            `json:"login"`
		Status  any            `json:"status"`
		Tools   any            `json:"tools"`
		Runs    []struct {
			Spec struct {
				Cwd, Mode, ResumeSessionID, Model string
			} `json:"spec"`
			Cmd any `json:"cmd"`
		} `json:"runs"`
	} `json:"commands"`
	HostEnv map[string]string `json:"hostEnv"`
}

func load(t *testing.T) adaptersGolden {
	raw, err := os.ReadFile("testdata/adapters.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g adaptersGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Claude) < 30 || len(g.Codex) < 4 || len(g.Outcomes) < 38 || len(g.Auth) < 13 || len(g.Tools) < 3 || len(g.Commands) < 5 || g.Commands[0].Login == nil || g.Auth[0].Status == nil {
		t.Fatalf("golden file not fully decoded: %d %d %d %d %d %d", len(g.Claude), len(g.Codex), len(g.Outcomes), len(g.Auth), len(g.Tools), len(g.Commands))
	}
	return g
}

func maps(evs []Event) []any {
	out := []any{}
	for _, e := range evs {
		out = append(out, e.Map())
	}
	return out
}

func check(t *testing.T, path string, got, want any) {
	t.Helper()
	if d := testutil.Diff(path, testutil.Generic(t, got), testutil.Generic(t, want)); d != "" {
		t.Error(d)
	}
}

func TestParsersMatchTypeScript(t *testing.T) {
	g := load(t)
	for i, c := range g.Claude {
		check(t, fmt.Sprintf("claude[%d] %.60s", i, c.Line), maps(ParseClaudeLine(c.Line)), c.Events)
	}
	for i, seq := range g.Codex {
		p := &CodexParser{}
		for j, c := range seq {
			check(t, fmt.Sprintf("codex[%d][%d] %.60s", i, j, c.Line), maps(p.Parse(c.Line)), c.Events)
		}
	}
}

func f64(v any) *float64 {
	if x, ok := v.(float64); ok {
		return &x
	}
	return nil
}

// eventFrom builds an Event from the TypeScript object.
func eventFrom(m map[string]any) Event {
	e := Event{Type: m["type"].(string)}
	s := func(k string) string { v, _ := m[k].(string); return v }
	switch e.Type {
	case EvSession:
		e.SessionID = str(s("sessionId"))
		e.Model = s("model")
	case EvMessage:
		e.Text = s("text")
	case EvRetry:
		a, _ := m["attempt"].(float64)
		e.Attempt, e.Error = int(a), s("error")
	case EvLimits:
		e.Status = s("status")
		raw, _ := json.Marshal(m["windows"])
		json.Unmarshal(raw, &e.Windows)
	case EvUsage:
		e.CostUSD, e.InputTokens, e.OutputTokens = f64(m["costUsd"]), f64(m["inputTokens"]), f64(m["outputTokens"])
	case EvResult:
		e.OK, _ = m["ok"].(bool)
		e.Text = s("text")
		if _, ok := m["sessionId"]; ok {
			e.SessionID = str(s("sessionId"))
		}
		e.StopReason = s("stopReason")
		d, _ := m["permissionDenials"].(float64)
		e.PermissionDenials = int(d)
		if _, ok := m["error"]; ok {
			e.Error, e.HasError = s("error"), true
		}
	}
	return e
}

func TestDecideOutcomeMatchesTypeScript(t *testing.T) {
	g := load(t)
	for i, c := range g.Outcomes {
		events := make([]Event, len(c.Events))
		for j, m := range c.Events {
			events[j] = eventFrom(m)
		}
		check(t, fmt.Sprintf("outcome[%d] cancel=%v", i, c.Cancel), DecideOutcome(events, c.Cancel).Map(), c.Out)
	}
}

func adapter(t *testing.T, app string) Adapter {
	if app == "claude" {
		app = domain.AppClaudeCode
	}
	a, ok := For(app)
	if !ok {
		t.Fatalf("no adapter for %s", app)
	}
	return a
}

func TestAuthAndToolsMatchTypeScript(t *testing.T) {
	g := load(t)
	for i, c := range g.Auth {
		check(t, fmt.Sprintf("auth[%d] %s", i, c.App), adapter(t, c.App).ParseAuthStatus(c.Stdout, c.Stderr, c.Code), c.Status)
	}
	for i, c := range g.Tools {
		check(t, fmt.Sprintf("tools[%d] %s", i, c.App), adapter(t, c.App).ParseToolList(c.Stdout), c.Tools)
	}
}

// commandMap renders a launchers.Command like the TypeScript object.
func commandMap(c launchers.Command) map[string]any {
	m := map[string]any{"executable": c.Executable, "args": c.Args, "env": c.Env, "unsetEnv": c.UnsetEnv}
	if c.UnsetEnv == nil {
		m["unsetEnv"] = []string{}
	}
	if c.Cwd != "" {
		m["cwd"] = c.Cwd
	}
	return m
}

func TestCommandsMatchTypeScript(t *testing.T) {
	g := load(t)
	for i, c := range g.Commands {
		p := c.Profile
		p.Normalize()
		a := adapter(t, p.App)
		env, unset := a.ProfileEnv(&p, g.HostEnv)
		if unset == nil {
			unset = []string{}
		}
		name := fmt.Sprintf("commands[%d] %s", i, p.App)
		check(t, name+" env", map[string]any{"env": env, "unsetEnv": unset}, c.Env)
		check(t, name+" login", commandMap(a.LoginCommand(&p, g.HostEnv)), c.Login)
		check(t, name+" status", commandMap(a.AuthStatusCommand(&p, g.HostEnv)), c.Status)
		check(t, name+" tools", commandMap(a.ListToolsCommand(&p, g.HostEnv)), c.Tools)
		for j, r := range c.Runs {
			spec := RunSpec{Cwd: r.Spec.Cwd, Mode: r.Spec.Mode, ResumeSessionID: r.Spec.ResumeSessionID, Model: r.Spec.Model}
			check(t, fmt.Sprintf("%s run[%d]", name, j), commandMap(a.RunCommand(&p, spec, g.HostEnv)), r.Cmd)
		}
	}
}
