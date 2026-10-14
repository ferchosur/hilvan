package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
)

// /profile lists the profiles, refuses unknown ids, and remembers the choice for the next session
// (D-043, FR-027). No model is called: the chat loop only handles local commands.
func TestChatProfileIsRememberedAcrossSessions(t *testing.T) {
	tmp := t.TempDir()
	host := core.CurrentHost()
	host.InsideWSL = false
	platform := config.Platform{OS: host.OS, Env: map[string]string{"HILVAN_CONFIG_DIR": filepath.Join(tmp, "config")}, Homedir: tmp}
	newApp := func(input string) *App {
		return &App{Stdin: strings.NewReader(input), Cwd: tmp, Platform: platform, Host: host}
	}
	if _, errOut, code := run(t, newApp(""), "profile", "add", "--id", "fake", "--app", "claude-code", "--accepts", "personal",
		"--executable", os.Args[0], "--config-dir", filepath.Join(tmp, "cfg")); code != 0 {
		t.Fatalf("profile add: %s", errOut)
	}

	out, errOut, code := run(t, newApp("/profile\n/profile nope\n/profile fake\n/exit\n"), "chat")
	if code != 0 {
		t.Fatalf("chat: %s", errOut)
	}
	must(t, `fake\s+claude-code @ native`, out)
	must(t, `no profile with id "nope"`, errOut)
	must(t, `remembered for next time`, out)

	out, _, code = run(t, newApp("/profile\n/exit\n"), "chat")
	if code != 0 {
		t.Fatal("second chat session failed")
	}
	must(t, `\* fake`, out)
}

// Free text is not interpreted yet; it must not fail or call anything.
func TestChatFreeTextIsNotSent(t *testing.T) {
	tmp := t.TempDir()
	host := core.CurrentHost()
	host.InsideWSL = false
	platform := config.Platform{OS: host.OS, Env: map[string]string{"HILVAN_CONFIG_DIR": filepath.Join(tmp, "config")}, Homedir: tmp}
	app := &App{Stdin: strings.NewReader("lee README.md\n/exit\n"), Cwd: tmp, Platform: platform, Host: host}
	out, _, code := run(t, app, "chat")
	if code != 0 || !strings.Contains(out, "not available yet") {
		t.Errorf("free text: code %d, %q", code, out)
	}
}

// /interpreter is chosen apart from /profile (D-042), remembered (D-043), and the list marks the
// default interpreter (routing default, highest preference) until one is chosen.
func TestChatInterpreterIsSeparateAndRemembered(t *testing.T) {
	tmp := t.TempDir()
	host := core.CurrentHost()
	host.InsideWSL = false
	platform := config.Platform{OS: host.OS, Env: map[string]string{"HILVAN_CONFIG_DIR": filepath.Join(tmp, "config")}, Homedir: tmp}
	newApp := func(input string) *App {
		return &App{Stdin: strings.NewReader(input), Cwd: tmp, Platform: platform, Host: host}
	}
	for _, args := range [][]string{
		{"profile", "add", "--id", "fake", "--app", "claude-code", "--accepts", "personal", "--executable", os.Args[0], "--config-dir", filepath.Join(tmp, "a")},
		{"profile", "add", "--id", "other", "--app", "codex", "--accepts", "personal", "--executable", os.Args[0], "--config-dir", filepath.Join(tmp, "b")},
	} {
		if _, errOut, code := run(t, newApp(""), args...); code != 0 {
			t.Fatalf("%v: %s", args, errOut)
		}
	}

	out, _, _ := run(t, newApp("/interpreter\n/exit\n"), "chat")
	must(t, `\* fake[^\n]*\n[^\n]*\n[^\n]*default interpreter until you choose`, out)
	out, _, _ = run(t, newApp("/interpreter other\n/profile fake\n/exit\n"), "chat")
	must(t, `sentences will be read by other`, out)
	must(t, `tasks will run with fake`, out)

	out, _, _ = run(t, newApp("/interpreter\n/profile\n/exit\n"), "chat")
	must(t, `\* other`, out)
	must(t, `\* fake`, out)
}
