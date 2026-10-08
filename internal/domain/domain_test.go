package domain

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func str(s string) *string { return &s }

func baseProfile() Profile {
	p := Profile{
		ID:          "claude-corp",
		Label:       "Claude — corporate",
		App:         AppClaudeCode,
		Auth:        Auth{Mode: "subscription"},
		ConfigDir:   str("/home/user/.hilvan-profiles/claude-corp"),
		Destination: Destination{Launcher: LauncherNative},
		Accepts:     []string{"corporate"},
	}
	p.Normalize()
	return p
}

func TestProfileDefaults(t *testing.T) {
	p := baseProfile()
	if len(p.Validate()) != 0 {
		t.Fatalf("unexpected issues: %v", p.Validate())
	}
	if len(p.Tags) != 0 || len(p.Tools.Provides) != 0 || p.Model != nil || p.Preference != 0 {
		t.Fatalf("defaults not applied: %+v", p)
	}
	if p.Tools.MCP != "account" || p.Routing != "default" {
		t.Fatalf("mcp=%q routing=%q", p.Tools.MCP, p.Routing)
	}
}

func TestProfileRules(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Profile)
		valid  bool
	}{
		{"wsl requires a distro", func(p *Profile) { p.Destination.Launcher = LauncherWSL }, false},
		{"wsl with distro", func(p *Profile) { p.Destination = Destination{Launcher: LauncherWSL, Distro: "Ubuntu"} }, true},
		{"api-key names a variable", func(p *Profile) { p.Auth = Auth{Mode: "api-key", APIKeyEnv: "WORK_KEY"} }, true},
		{"api-key never holds a value", func(p *Profile) { p.Auth = Auth{Mode: "api-key", APIKeyEnv: "sk-ant-123"} }, false},
		{"unknown app", func(p *Profile) { p.App = "other" }, false},
		{"invalid id", func(p *Profile) { p.ID = "Bad Id" }, false},
		{"mcp profile needs a config", func(p *Profile) { p.Tools.MCP = "profile" }, false},
	}
	for _, c := range cases {
		p := baseProfile()
		c.mutate(&p)
		if got := len(p.Validate()) == 0; got != c.valid {
			t.Errorf("%s: valid=%v, want %v (%v)", c.name, got, c.valid, p.Validate())
		}
	}
}

func TestProfilesFile(t *testing.T) {
	empty := ProfilesFile{Version: 1}
	empty.Normalize()
	if len(empty.Validate()) != 0 || len(empty.Profiles) != 0 {
		t.Fatal("an empty file must be valid with no profiles")
	}
	other := baseProfile()
	other.ID, other.Accepts = "claude-personal", []string{"personal"}
	two := ProfilesFile{Version: 1, Profiles: []Profile{baseProfile(), other}}
	if len(two.Validate()) != 0 {
		t.Fatal("several profiles of the same app are allowed (AC-011)")
	}
	dup := ProfilesFile{Version: 1, Profiles: []Profile{baseProfile(), baseProfile()}}
	issues := dup.Validate()
	if len(issues) != 1 || !strings.Contains(issues.Pretty(), `✖ duplicate id "claude-corp"`+"\n  → at profiles[1].id") {
		t.Fatalf("duplicate ids: %q", issues.Pretty())
	}
	v2 := ProfilesFile{Version: 2}
	if got := v2.Validate().Pretty(); got != "✖ Invalid input: expected 1\n  → at version" {
		t.Fatalf("version message: %q", got)
	}
}

func TestTaskStateMachine(t *testing.T) {
	checks := []struct {
		from, to TaskState
		ok       bool
	}{
		{TaskRunning, TaskBlocked, true},
		{TaskBlocked, TaskRunning, false}, // never moves to another profile silently
		{TaskBlocked, TaskReady, true},
		{TaskRunning, TaskAccepted, false}, // review comes first
		{TaskInReview, TaskAccepted, true},
	}
	for _, c := range checks {
		if CanTransition(c.from, c.to) != c.ok {
			t.Errorf("%s → %s: want %v", c.from, c.to, c.ok)
		}
	}
	if !IsTerminal(TaskAccepted) || !IsTerminal(TaskDiscarded) || IsTerminal(TaskUncertain) {
		t.Fatal("terminal states")
	}
}

func TestISODateTimeMatchesZod(t *testing.T) {
	cases := map[string]bool{
		"2026-10-07T00:00:00.000Z":       true,
		"2026-10-07T00:00Z":              false,
		"2026-10-07T00:00:00Z":           true,
		"2026-02-30T00:00:00Z":           false,
		"2026-10-07T00:00:00+02:00":      false,
		"2026-10-07T24:00:00Z":           false,
		"2026-10-07T00:00:00.123456789Z": true,
		"2026-10-07 00:00:00Z":           false,
		"2024-02-29T00:00:00Z":           true,
		"2023-02-29T00:00:00Z":           false,
	}
	for s, want := range cases {
		if IsISODateTime(s) != want {
			t.Errorf("%s: want %v", s, want)
		}
	}
}

func TestULID(t *testing.T) {
	at := time.UnixMilli(1_700_000_000_000)
	var zero, ones [10]byte
	for i := range ones {
		ones[i] = 0xff
	}
	a, b := ULID(at, zero), ULID(at, ones)
	if len(a) != 26 || a[10:] != "0000000000000000" || b[10:] != "ZZZZZZZZZZZZZZZZ" || a[:10] != b[:10] {
		t.Fatalf("ulid: %s %s", a, b)
	}
	if !taskIDPattern.MatchString(NewTaskID()) || !runIDPattern.MatchString(NewRunID()) || !handoffIDPattern.MatchString(NewHandoffID()) {
		t.Fatal("prefixed ids")
	}
	if ULID(at.Add(time.Millisecond), zero) <= a {
		t.Fatal("ULIDs must sort by time")
	}
}

func TestULIDMatchesTypeScript(t *testing.T) {
	raw, err := os.ReadFile("../jsfmt/testdata/text.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		ULIDs []struct {
			Now  int64  `json:"now"`
			Rand []byte `json:"-"`
			R    []int  `json:"rand"`
			Out  string `json:"out"`
		} `json:"ulids"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.ULIDs {
		var r [10]byte
		for i, b := range c.R {
			r[i] = byte(b)
		}
		if got := ULID(time.UnixMilli(c.Now), r); got != c.Out {
			t.Errorf("ulid(%d, %v) = %s, want %s", c.Now, c.R, got, c.Out)
		}
	}
}

// task.json written by Go must be byte-identical to the TypeScript output (plan 002).
func TestTaskJSONMatchesTypeScript(t *testing.T) {
	raw, err := os.ReadFile("testdata/tasks.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		In  json.RawMessage `json:"in"`
		Out string          `json:"out"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		var task Task
		if err := json.Unmarshal(c.In, &task); err != nil {
			t.Fatal(err)
		}
		task.Normalize()
		if issues := task.Validate(); len(issues) > 0 {
			t.Fatalf("case %d: %s", i, issues.Pretty())
		}
		var b strings.Builder
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(task); err != nil {
			t.Fatal(err)
		}
		if b.String() != c.Out {
			t.Errorf("case %d:\ngo:\n%s\nts:\n%s", i, b.String(), c.Out)
		}
	}
}
