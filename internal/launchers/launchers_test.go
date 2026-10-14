package launchers

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ferchosur/hilvan/internal/domain"
)

var cmd = Command{
	Executable: "claude",
	Args:       []string{"auth", "status", "--json"},
	Env:        map[string]string{"CLAUDE_CONFIG_DIR": "/home/u/.hilvan-profiles/a"},
	UnsetEnv:   []string{"ANTHROPIC_API_KEY"},
}

var linux = Host{OS: "linux", Env: map[string]string{"PATH": "/usr/bin", "ANTHROPIC_API_KEY": "stray", "CLAUDE_CONFIG_DIR": "/other"}}

func TestNative(t *testing.T) {
	inv, err := BuildInvocation(domain.Destination{Launcher: "native"}, cmd, linux)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Command != "claude" || !reflect.DeepEqual(inv.Args, cmd.Args) || inv.Env["CLAUDE_CONFIG_DIR"] != "/home/u/.hilvan-profiles/a" || inv.Env["PATH"] != "/usr/bin" {
		t.Fatalf("%+v", inv)
	}
	if _, ok := inv.Env["ANTHROPIC_API_KEY"]; ok {
		t.Error("stray provider key kept")
	}
	over, _ := BuildInvocation(domain.Destination{Launcher: "native", Executable: "/opt/claude"}, cmd, linux)
	if over.Command != "/opt/claude" {
		t.Error("executable override")
	}
}

func TestUnsetIsCaseInsensitiveOnWindows(t *testing.T) {
	win := Host{OS: "win32", Env: map[string]string{"Anthropic_Api_Key": "stray", "Path": `C:\x`}}
	inv, _ := BuildInvocation(domain.Destination{Launcher: "native"}, cmd, win)
	if _, ok := inv.Env["Anthropic_Api_Key"]; ok {
		t.Error("Windows variable names are case-insensitive")
	}
}

func TestWSL(t *testing.T) {
	win := Host{OS: "win32", Env: map[string]string{"Path": `C:\x`}}
	c := cmd
	c.Cwd = "/mnt/c/Users/u/Documents/claude/repo"
	inv, err := BuildInvocation(domain.Destination{Launcher: "wsl", Distro: "Ubuntu", Executable: "/home/u/.nvm/versions/node/v24/bin/claude", Path: "/home/u/.nvm/versions/node/v24/bin:/usr/bin:/bin"}, c, win)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-d", "Ubuntu", "--cd", "/mnt/c/Users/u/Documents/claude/repo", "--exec", "env",
		"-u", "ANTHROPIC_API_KEY", "PATH=/home/u/.nvm/versions/node/v24/bin:/usr/bin:/bin",
		"CLAUDE_CONFIG_DIR=/home/u/.hilvan-profiles/a", "/home/u/.nvm/versions/node/v24/bin/claude", "auth", "status", "--json"}
	if inv.Command != "wsl.exe" || !reflect.DeepEqual(inv.Args, want) {
		t.Fatalf("args:\n%q\n%q", inv.Args, want)
	}
	if _, ok := inv.Env["CLAUDE_CONFIG_DIR"]; ok {
		t.Error("profile variables must be set inside the distro, not on wsl.exe")
	}
	if _, err := BuildInvocation(domain.Destination{Launcher: "wsl", Distro: "U"}, cmd, linux); !errors.Is(err, ErrLauncher) {
		t.Error("wsl launcher outside Windows")
	}
}

func TestWindowsFromWSL(t *testing.T) {
	h := Host{OS: "linux", InsideWSL: true, Env: map[string]string{"WSLENV": "FOO", "WSL_DISTRO_NAME": "Ubuntu"}}
	c := cmd
	c.Env = map[string]string{"CLAUDE_CONFIG_DIR": `C:\Users\u\.hilvan-profiles\corp`}
	c.Cwd = `C:\Users\u\Documents\claude\repo`
	inv, err := BuildInvocation(domain.Destination{Launcher: "windows", Executable: "/mnt/c/Users/u/.local/bin/claude.exe"}, c, h)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Env["WSLENV"] != "FOO:CLAUDE_CONFIG_DIR" || inv.Env["CLAUDE_CONFIG_DIR"] != `C:\Users\u\.hilvan-profiles\corp` {
		t.Errorf("env %v", inv.Env)
	}
	if inv.Cwd != "/mnt/c/Users/u/Documents/claude/repo" {
		t.Errorf("host cwd %q", inv.Cwd)
	}
	if _, err := BuildInvocation(domain.Destination{Launcher: "windows"}, cmd, linux); !errors.Is(err, ErrLauncher) {
		t.Error("windows launcher outside WSL")
	}
}

func TestPathTranslation(t *testing.T) {
	toWSL := map[string]string{
		`C:\Users\u\Documents\claude\repo`:   "/mnt/c/Users/u/Documents/claude/repo",
		`D:\`:                                "/mnt/d",
		`c:/x/y`:                             "/mnt/c/x/y",
		`\\wsl.localhost\Ubuntu\home\u\repo`: "/home/u/repo",
		`\\wsl$\ubuntu\home\u`:               "/home/u",
		"/already/linux":                     "/already/linux",
	}
	for in, want := range toWSL {
		if got, err := WindowsToWSL(in, "Ubuntu"); err != nil || got != want {
			t.Errorf("WindowsToWSL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := WindowsToWSL(`\\wsl.localhost\Debian\x`, "Ubuntu"); err == nil {
		t.Error("path of another distro accepted")
	}
	if _, err := WindowsToWSL(`\\server\share\x`, "Ubuntu"); err == nil {
		t.Error("network share accepted")
	}
	toWin := map[string]string{
		"/mnt/c/Users/u/Documents/claude/repo": `C:\Users\u\Documents\claude\repo`,
		"/mnt/d":                               `D:\`,
		"/home/u/repo":                         `\\wsl.localhost\Ubuntu\home\u\repo`,
		"/mnt/data/x":                          `\\wsl.localhost\Ubuntu\mnt\data\x`,
		`C:\already`:                           `C:\already`,
	}
	for in, want := range toWin {
		if got, err := WSLToWindows(in, "Ubuntu"); err != nil || got != want {
			t.Errorf("WSLToWindows(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := WSLToWindows("/home/u", ""); err == nil {
		t.Error("Linux path without WSL_DISTRO_NAME accepted")
	}
	h := Host{OS: "win32"}
	if p, _ := DestinationPath(domain.Destination{Launcher: "wsl", Distro: "Ubuntu"}, h, `C:\r`); p != "/mnt/c/r" {
		t.Errorf("destination path %q", p)
	}
}
