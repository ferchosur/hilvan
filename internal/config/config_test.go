package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"hilvan/internal/criterio"
	"hilvan/internal/domain"
)

var (
	linux = Platform{OS: "linux", Env: map[string]string{}, Homedir: "/home/u"}
	win   = Platform{OS: "win32", Env: map[string]string{`APPDATA`: `C:\Users\u\AppData\Roaming`, "ProgramData": `C:\ProgramData`}, Homedir: `C:\Users\u`}
	mac   = Platform{OS: "darwin", Env: map[string]string{}, Homedir: "/Users/u"}
)

func with(p Platform, k, v string) Platform {
	env := map[string]string{k: v}
	for a, b := range p.Env {
		env[a] = b
	}
	p.Env = env
	return p
}

func TestPaths(t *testing.T) {
	checks := []struct{ got, want string }{
		{linux.UserConfigDir(), "/home/u/.config/hilvan"},
		{with(linux, "XDG_CONFIG_HOME", "/x").UserConfigDir(), "/x/hilvan"},
		{win.ProfilesFile(), `C:\Users\u\AppData\Roaming\hilvan\profiles.yaml`},
		{mac.UserConfigDir(), "/Users/u/Library/Application Support/hilvan"},
		{with(linux, "HILVAN_CONFIG_DIR", "/tmp/hv").UserConfigDir(), "/tmp/hv"},
		{linux.ManagedPolicyFile(), "/etc/hilvan/policy.yaml"},
		{win.ManagedPolicyFile(), `C:\ProgramData\hilvan\policy.yaml`},
		{Platform{OS: "win32", Env: map[string]string{}, Homedir: `C:\Users\u`}.UserConfigDir(), `C:\Users\u\AppData\Roaming\hilvan`},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("got %q want %q", c.got, c.want)
		}
	}
}

func TestParseProfiles(t *testing.T) {
	f, err := ParseProfiles("", "profiles.yaml")
	if err != nil || len(f.Profiles) != 0 {
		t.Fatalf("empty file: %v %v", f, err)
	}
	yaml := `
version: 1
profiles:
  - id: claude-corp
    label: Claude corporate (Windows)
    app: claude-code
    auth: { mode: subscription }
    configDir: 'C:\Users\u\.hilvan-profiles\claude-corp'
    destination: { launcher: native }
    accepts: [corporate, confidential]
    tags: [corporate]
    unknown: ignored like zod does
  - id: claude-personal
    label: Claude personal (WSL)
    app: claude-code
    auth: { mode: subscription }
    configDir: /home/u/.hilvan-profiles/claude-personal
    destination: { launcher: wsl, distro: Ubuntu-24.04 }
    accepts: [public, personal]
    tags: [personal]
`
	f, err = ParseProfiles(yaml, "profiles.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if f.Profiles[0].Destination.Launcher != "native" || f.Profiles[1].Destination.Launcher != "wsl" || *f.Profiles[0].ConfigDir != `C:\Users\u\.hilvan-profiles\claude-corp` {
		t.Fatalf("parsed: %+v", f.Profiles)
	}
	for _, bad := range []string{"version: 2", "version: [", "version: 1\nversion: 1"} {
		_, err := ParseProfiles(bad, "profiles.yaml")
		var ce *ConfigError
		if !errors.As(err, &ce) {
			t.Errorf("%q: want ConfigError, got %v", bad, err)
		}
	}
	_, err = ParseProfiles("version: 2", "profiles.yaml")
	if want := "Invalid configuration in profiles.yaml:\n✖ Invalid input: expected 1\n  → at version"; err.Error() != want {
		t.Errorf("message %q", err.Error())
	}
}

func TestAliasBombIsRejected(t *testing.T) {
	levels := []string{`a: &a ["x","x","x","x","x","x","x","x","x","x"]`}
	for i := 1; i < 12; i++ {
		prev, cur := string(rune('a'+i-1)), string(rune('a'+i))
		refs := strings.TrimSuffix(strings.Repeat("*"+prev+",", 10), ",")
		levels = append(levels, fmt.Sprintf("%s: &%s [%s]", cur, cur, refs))
	}
	var ce *ConfigError
	if _, err := ParseProfiles(strings.Join(levels, "\n"), "x"); !errors.As(err, &ce) {
		t.Fatalf("alias bomb: %v", err)
	}
}

func TestProfilesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "cfg", "profiles.yaml")
	cfg := "/h/a"
	data := domain.ProfilesFile{Version: 1, Profiles: []domain.Profile{{
		ID: "a", Label: "A: \"quoted\" #1", App: "codex", Auth: domain.Auth{Mode: "subscription"},
		ConfigDir: &cfg, Destination: domain.Destination{Launcher: "native"}, Accepts: []string{"public"},
		Tools: domain.Tools{Provides: []string{"jira"}, Aliases: domain.Aliases{"jira": {"ticket", "2026-10-07"}}},
	}}}
	if err := SaveProfiles(file, data); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(file)
	if !strings.HasPrefix(string(raw), "# Hilvan profiles") {
		t.Error("header missing")
	}
	back, err := LoadProfiles(file)
	if err != nil {
		t.Fatal(err)
	}
	p := back.Profiles[0]
	if p.Label != data.Profiles[0].Label || p.Tools.Aliases["jira"][1] != "2026-10-07" || p.Tools.MCP != "account" || p.Model != nil {
		t.Fatalf("round trip: %+v\n%s", p, raw)
	}
	missing, err := LoadProfiles(filepath.Join(dir, "none.yaml"))
	if err != nil || missing.Version != 1 || len(missing.Profiles) != 0 {
		t.Fatalf("missing file: %v %v", missing, err)
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link.yaml")
		os.Symlink(file, link)
		var ce *ConfigError
		if _, err := LoadProfiles(link); !errors.As(err, &ce) {
			t.Errorf("symlinked profiles: %v", err)
		}
	}
}

func TestPolicyFiles(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user.yaml")
	os.WriteFile(user, []byte("version: 1\nrules:\n  - id: r1\n    from: { classification: [corporate] }\n    to: { tags: [personal] }\n    effect: require_approval\n    reason: needs approval\n"), 0o600)
	eff, err := LoadEffectivePolicy(PolicyPaths{Managed: filepath.Join(dir, "none.yaml"), User: user})
	if err != nil {
		t.Fatal(err)
	}
	if len(eff.Rules) != 1 || eff.Rules[0].Layer+":"+eff.Rules[0].ID != "user:r1" {
		t.Fatalf("rules: %+v", eff.Rules)
	}
	for name, doc := range map[string]string{
		"allow effect":  "version: 1\nrules:\n  - id: r2\n    effect: allow\n    reason: x\n",
		"unknown field": "version: 1\nrules:\n  - id: r3\n    effect: deny\n    reason: x\n    priority: 1\n",
	} {
		f := filepath.Join(dir, "bad.yaml")
		os.WriteFile(f, []byte(doc), 0o600)
		if _, err := LoadPolicyFile(f); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestCriterioPersistence(t *testing.T) {
	dir := t.TempDir()
	c := criterio.New()
	c.LearnTaskType(criterio.TaskSignals{Title: "Spike secreto proyecto Atlas"}, "research")
	c.RecordOutcome("p1", "research", true)
	file := CriterioFile(dir)
	if err := SaveCriterio(file, c); err != nil {
		t.Fatal(err)
	}
	engine, warning, err := LoadCriterio(file)
	if err != nil || warning != "" {
		t.Fatalf("load: %v %q", err, warning)
	}
	a, _ := json.Marshal(engine.State())
	b, _ := json.Marshal(c.State())
	if string(a) != string(b) {
		t.Error("state changed after save and load")
	}
	bad := filepath.Join(dir, "bad.json")
	os.WriteFile(bad, []byte(`{"version":1,"taskType":42}`), 0o600)
	if _, w, _ := LoadCriterio(bad); !strings.Contains(w, "invalid") {
		t.Errorf("corrupted model: %q", w)
	}
	big := filepath.Join(dir, "big.json")
	os.WriteFile(big, []byte(strings.Repeat(" ", MaxModelBytes+1)), 0o600)
	if _, w, _ := LoadCriterio(big); !strings.Contains(w, "too large") {
		t.Errorf("oversized model: %q", w)
	}
	if _, w, _ := LoadCriterio(filepath.Join(dir, "missing.json")); w != "" {
		t.Errorf("missing model warned: %q", w)
	}
}
