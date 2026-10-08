package security

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"CSI colour", "ok \x1b[31mred\x1b[0m", "ok red"},
		{"cursor movement", "a\x1b[2Kb\x1b[1A", "ab"},
		{"OSC 52 clipboard write", "x\x1b]52;c;ZXZpbA==\x07y", "xy"},
		{"OSC 8 hyperlink", "\x1b]8;;http://evil\x1b\\click\x1b]8;;\x1b\\", "click"},
		{"DCS", "a\x1bPq#0;2;0;0;0\x1b\\b", "ab"},
		{"8-bit CSI", "a\u009b31mb", "ab"},
		{"bell and backspace", "a\x07\x08b", "ab"},
		{"bidi override (Trojan Source)", "admin‮⁦ // user", "admin // user"},
		{"zero width", "pa​ss", "pass"},
	}
	for _, c := range cases {
		if got := SanitizeForTerminal(c.in, SanitizeOptions{}); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
		if !HasControlSequences(c.in) {
			t.Errorf("%s: control sequences not detected", c.name)
		}
	}
	if got := SanitizeForTerminal("a\n\tb\r\nc", SanitizeOptions{}); got != "a\n\tb\nc" {
		t.Errorf("multiline: %q", got)
	}
	if got := SanitizeForTerminal("a\nb", SanitizeOptions{SingleLine: true}); got != "a b" {
		t.Errorf("single line: %q", got)
	}
	if got := SanitizeForTerminal("abcdef", SanitizeOptions{MaxLength: 4}); got != "abc…" {
		t.Errorf("max length: %q", got)
	}
	if HasControlSequences("plain text\nok") {
		t.Error("plain text flagged")
	}
}

func kinds(ms []SecretMatch) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Kind
	}
	return out
}

func TestSecrets(t *testing.T) {
	cases := []struct{ kind, text string }{
		{"anthropic-key", "key sk-ant-api03-" + strings.Repeat("a1B2", 10)},
		{"openai-key", "OPENAI=sk-proj-" + strings.Repeat("Zx9", 12)},
		{"github-token", "ghp_" + strings.Repeat("A1b2C3d4", 5)},
		{"aws-access-key", "AKIAIOSFODNN7EXAMPLE"},
		{"private-key", "-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----"},
		{"url-credentials", "postgres://admin:hunter2pw@db.internal:5432/app"},
		{"assignment", `password = "correcthorse99"`},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abcdefghijklmnopqrstu"},
	}
	for _, c := range cases {
		if !strings.Contains(strings.Join(kinds(FindSecrets(c.text)), ","), c.kind) {
			t.Errorf("%s not detected in %q: %v", c.kind, c.text, kinds(FindSecrets(c.text)))
		}
	}
	if got := kinds(FindSecrets("token Xk9mQ2vL7pR4sT8wZ1yB6nC3dF5gH0jK")); len(got) != 1 || got[0] != "high-entropy" {
		t.Errorf("high entropy: %v", got)
	}
	for _, s := range []string{"src/components/navigation/TopBarMenuItem.tsx", "commit 4e6559850ad2c3b1a9f0e8d7c6b5a4f3e2d1c0b9"} {
		if ms := FindSecrets(s); len(ms) != 0 {
			t.Errorf("false positive in %q: %v", s, kinds(ms))
		}
	}
	if Entropy("aaaa") != 0 {
		t.Error("entropy of a constant string")
	}
	text, matches := RedactSecrets("use AKIAIOSFODNN7EXAMPLE now")
	if text != "use [REDACTED:aws-access-key] now" || len(matches) != 1 {
		t.Errorf("redact: %q %v", text, matches)
	}
}

func TestArgs(t *testing.T) {
	if AssertSafeArgs([]string{"ok", "a\nb"}) == nil || AssertSafeArgs([]string{"a\x00"}) == nil {
		t.Error("newline/NUL accepted")
	}
	if AssertSafeArgs([]string{"--flag=value with spaces"}) != nil {
		t.Error("spaces rejected")
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "sub", "a.yaml")
	for _, s := range []string{"one", "two"} {
		if err := WriteFileAtomic(f, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := os.ReadFile(f); string(b) != "two" {
		t.Fatalf("content %q", b)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(f); st.Mode().Perm() != 0o600 {
			t.Errorf("file mode %o", st.Mode().Perm())
		}
		if st, _ := os.Stat(filepath.Dir(f)); st.Mode().Perm() != 0o700 {
			t.Errorf("dir mode %o", st.Mode().Perm())
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(f))
	if len(entries) != 1 || entries[0].Name() != "a.yaml" {
		t.Errorf("temp files left: %v", entries)
	}
}

func TestSymlinksAndLimits(t *testing.T) {
	dir := t.TempDir()
	big := filepath.Join(dir, "big")
	os.WriteFile(big, []byte(strings.Repeat("x", 100)), 0o600)
	if _, _, err := ReadTextSafe(big, 10); !IsUnsafePath(err) {
		t.Errorf("size limit: %v", err)
	}
	if _, found, err := ReadTextSafe(filepath.Join(dir, "nope"), 10); found || err != nil {
		t.Errorf("missing file: found=%v err=%v", found, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	target := filepath.Join(dir, "victim.txt")
	os.WriteFile(target, []byte("precious"), 0o600)
	link := filepath.Join(dir, "profiles.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadTextSafe(link, 1000); !IsUnsafePath(err) {
		t.Errorf("read through symlink: %v", err)
	}
	if err := WriteFileAtomic(link, []byte("pwned"), 0o600); !IsUnsafePath(err) {
		t.Errorf("write through symlink: %v", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "precious" {
		t.Error("symlink target modified")
	}
	cfg := filepath.Join(dir, "cfg.yaml")
	os.WriteFile(cfg, []byte("version: 1"), 0o600)
	os.Chmod(cfg, 0o666)
	if msg := LooseConfigPermissions(cfg); !strings.Contains(msg, "writable by group/others") {
		t.Errorf("loose permissions: %q", msg)
	}
	os.Chmod(cfg, 0o600)
	if msg := LooseConfigPermissions(cfg); msg != "" {
		t.Errorf("tight permissions flagged: %q", msg)
	}
}
