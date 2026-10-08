package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/process"
	"github.com/ferchosur/hilvan/internal/testutil"
)

func TestMain(m *testing.M) {
	testutil.RunFakeAgentIfRequested()
	os.Exit(m.Run())
}

func TestParsing(t *testing.T) {
	var got *Ctx
	cmd := (&Command{Name: "hilvan"}).Add((&Command{Name: "task"}).Add(&Command{
		Name: "new",
		Args: []string{"<title>", "[extra]"},
		Options: []*Option{
			{Flags: "--criteria <text>", Collect: true},
			{Flags: "--mode <mode>", Default: "write"},
			{Flags: "-p, --profile <id>"},
			{Flags: "--json"},
			{Flags: "--requires <list>"},
		},
		Run: func(c *Ctx) error { got = c; return nil },
	}))
	_, ctx, err := cmd.dispatch([]string{"task", "new", "--criteria", "a", "Title", "--criteria=b c", "-p", "x", "--json", "--requires", "jira, wiki,,"})
	if err != nil {
		t.Fatal(err)
	}
	ctx.App = &App{}
	_ = cmd.Subs[0].Subs[0].Run(ctx)
	if got.Args[0] != "Title" || len(got.Args) != 1 {
		t.Errorf("args %v", got.Args)
	}
	if v := got.All("criteria"); len(v) != 2 || v[1] != "b c" {
		t.Errorf("repeatable %v", v)
	}
	if got.Str("mode") != "write" || got.Str("profile") != "x" || !got.Bool("json") {
		t.Errorf("values %v", got.values)
	}
	if l := got.List("requires"); len(l) != 2 || l[1] != "wiki" {
		t.Errorf("list %v", l)
	}
	if got.List("nope") != nil {
		t.Error("absent list must be nil")
	}
	for args, want := range map[string]string{
		"task new":             "error: missing required argument 'title'",
		"task new a b c":       "error: too many arguments for 'new'. Expected 2 arguments but got 3: a, b, c.",
		"task new a --zzz":     "error: unknown option '--zzz'",
		"task new a --mode":    "error: option '--mode <mode>' argument missing",
		"task frob":            "error: unknown command 'frob'",
		"task new -- --mode x": "",
	} {
		_, _, err := cmd.dispatch(strings.Fields(args))
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		if msg != want {
			t.Errorf("%q: %q, want %q", args, msg, want)
		}
	}
}

func run(t *testing.T, app *App, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	app.Stdout, app.Stderr = &out, &errOut
	code := Main(app, args)
	return out.String(), errOut.String(), code
}

func must(t *testing.T, re, s string) {
	t.Helper()
	if !regexp.MustCompile(re).MatchString(s) {
		t.Fatalf("%q does not match %s", s, re)
	}
}

// A whole session through Main: profiles, init, task, run with the fake agent, review, accept.
func TestEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	if out, err := exec.Command("git", "init", "-q", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "i").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	host := core.CurrentHost()
	host.InsideWSL = false
	host.Env = testutil.FakeAgentEnv(process.CurrentEnv(), "write")
	platform := config.Platform{OS: host.OS, Env: map[string]string{"HILVAN_CONFIG_DIR": filepath.Join(tmp, "config")}, Homedir: tmp}
	app := &App{Stdin: strings.NewReader(""), Cwd: repo, Platform: platform, Host: host}

	out, _, code := run(t, app, "profile", "add", "--id", "fake", "--app", "claude-code", "--accepts", "personal", "--executable", os.Args[0], "--config-dir", filepath.Join(tmp, "cfg"))
	if code != 0 {
		t.Fatalf("profile add: %s", out)
	}
	must(t, `✔ profile fake added \(claude-code @ native\)`, out)
	out, _, _ = run(t, app, "profile", "list")
	must(t, `(?m)^ID\s+APP\s+DESTINATION`, out)
	must(t, `(?m)^fake\s+claude-code\s+native\s+subscription\s+personal`, out)

	out, _, code = run(t, app, "init")
	if code != 0 {
		t.Fatalf("init: %s", out)
	}
	out, _, _ = run(t, app, "task", "new", "Add hello file", "--criteria", "hello.txt exists")
	must(t, `✔ task \w{6} created: Add hello file`, out)
	id := regexp.MustCompile(`task (\w{6}) created`).FindStringSubmatch(out)[1]

	out, errOut, code := run(t, app, "task", "run", id, "-p", "fake")
	if code != 0 {
		t.Fatalf("run: %s %s", out, errOut)
	}
	must(t, `✔ completed · profile fake · ~\$0\.0123 estimated`, out)
	must(t, `task is now in_review`, out)
	if strings.Contains(out, "\x1b") {
		t.Error("agent output reached the terminal unsanitised (SR-001)")
	}

	_, errOut, code = run(t, app, "accept", id)
	if code != 1 || !strings.Contains(errOut, "review the current diff first") {
		t.Errorf("accept before review: %d %s", code, errOut)
	}
	out, _, _ = run(t, app, "review", id)
	must(t, `hello from the agent`, out)
	out, _, code = run(t, app, "accept", id)
	if code != 0 {
		t.Fatalf("accept: %s", out)
	}
	must(t, `accepted and merged into main`, out)
	out, _, code = run(t, app, "audit", "verify")
	if code != 0 {
		t.Fatalf("audit: %s", out)
	}
	must(t, `✔ audit log intact \(\d+ events\)`, out)
	out, _, _ = run(t, app, "stats")
	must(t, `fake: 1 runs \(completed 1\)`, out)
}
