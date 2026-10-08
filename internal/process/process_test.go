package process

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ferchosur/hilvan/internal/security"
)

// The test binary doubles as a helper program: with HILVAN_TEST_HELPER set it prints its
// arguments as JSON (print) or sleeps (sleep) instead of running tests.
func TestMain(m *testing.M) {
	switch os.Getenv("HILVAN_TEST_HELPER") {
	case "print":
		json.NewEncoder(os.Stdout).Encode(os.Args[1:])
		fmt.Fprint(os.Stderr, "to stderr")
		os.Exit(3)
	case "sleep":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helperEnv(mode string) map[string]string {
	env := CurrentEnv()
	env["HILVAN_TEST_HELPER"] = mode
	return env
}

func TestLineSplitter(t *testing.T) {
	var lines []string
	s := NewLineSplitter(func(l string) { lines = append(lines, l) })
	s.Push("{\"a\":1}\n{\"b\"")
	s.Push(":2}\r\n\n{\"c\":3}")
	s.End()
	if want := []string{`{"a":1}`, `{"b":2}`, `{"c":3}`}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines %v", lines)
	}
	long := &LineSplitter{OnLine: func(string) {}, MaxLine: 10, MaxTotal: DefaultMaxTotal}
	if err := long.Push(strings.Repeat("x", 11)); !IsLineLimit(err) {
		t.Errorf("line limit: %v", err)
	}
	total := &LineSplitter{OnLine: func(string) {}, MaxLine: DefaultMaxLine, MaxTotal: 20}
	if err := total.Push(strings.Repeat("y\n", 11)); !IsLineLimit(err) {
		t.Errorf("total limit: %v", err)
	}
}

func TestRun(t *testing.T) {
	r, err := Run(os.Args[0], []string{"a b", "c"}, Options{Env: helperEnv("print")})
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	json.Unmarshal([]byte(r.Stdout), &args)
	if r.Code != 3 || !reflect.DeepEqual(args, []string{"a b", "c"}) || r.Stderr != "to stderr" {
		t.Fatalf("result %+v", r)
	}
	slow, _ := Run(os.Args[0], nil, Options{Env: helperEnv("sleep"), Timeout: 200 * time.Millisecond})
	if slow.Err != "timeout" {
		t.Errorf("timeout: %+v", slow)
	}
	missing, _ := Run(filepath.Join(t.TempDir(), "nope"), nil, Options{})
	if missing.Err != "not_found" {
		t.Errorf("missing executable: %+v", missing)
	}
	if _, err := Run("echo", []string{"x\ny"}, Options{}); !errors.Is(err, security.ErrUnsafeArgument) {
		t.Errorf("newline argument: %v", err)
	}
}

func TestDecodeOutput(t *testing.T) {
	msg := "wsl: Se detectó una configuración de proxy"
	var b []byte
	for _, r := range msg {
		b = append(b, byte(r), byte(r>>8))
	}
	if got := DecodeOutput(b); got != msg {
		t.Errorf("UTF-16LE without BOM: %q", got)
	}
	if got := DecodeOutput(append([]byte{0xff, 0xfe}, b...)); got != msg {
		t.Errorf("UTF-16LE with BOM: %q", got)
	}
	if got := DecodeOutput([]byte(`{"type":"result"}`)); got != `{"type":"result"}` {
		t.Errorf("UTF-8 changed: %q", got)
	}
}

func TestBatchCommandLine(t *testing.T) {
	line, err := BatchCommandLine(`C:\npm\codex.cmd`, []string{"exec", "--cd", `C:\Program Files (x86)\repo\`, "a&b|c"})
	if err != nil {
		t.Fatal(err)
	}
	want := `/d /s /c ""C:\npm\codex.cmd" "exec" "--cd" "C:\Program Files (x86)\repo\\" "a&b|c""`
	if line != want {
		t.Errorf("got  %s\nwant %s", line, want)
	}
	for _, bad := range []string{"%PATH%", `say "hi"`, "wow!", "a\nb"} {
		if _, err := BatchCommandLine("x.cmd", []string{bad}); !errors.Is(err, ErrUnsafeBatchArgument) {
			t.Errorf("%q accepted", bad)
		}
	}
}

// On Windows, a real .cmd shim receives tricky arguments intact and cannot be injected into.
func TestBatchShimOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows only")
	}
	dir := t.TempDir()
	shim := filepath.Join(dir, "shim.cmd")
	marker := filepath.Join(dir, "pwned.txt")
	os.WriteFile(shim, []byte("@\""+os.Args[0]+"\" %*\r\n"), 0o700)
	args := []string{"a&b", "x|y", "(p)", `C:\Program Files (x86)\x`, `end\`, "with space", "^caret", "<in>", "& echo pwned > " + marker}
	r, err := Run(shim, args, Options{Env: helperEnv("print")})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal([]byte(r.Stdout), &got); err != nil {
		t.Fatalf("output %q (stderr %q): %v", r.Stdout, r.Stderr, err)
	}
	if !reflect.DeepEqual(got, args) {
		t.Errorf("arguments changed:\ngot  %q\nwant %q", got, args)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("command injection through the shim")
	}
	if _, err := Run(shim, []string{"%PATH%"}, Options{}); !errors.Is(err, ErrUnsafeBatchArgument) {
		t.Errorf("percent argument: %v", err)
	}
}
