package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/process"
	"github.com/ferchosur/hilvan/internal/store"
	"github.com/ferchosur/hilvan/internal/testutil"
)

// The test binary doubles as the agent CLI (testutil.FakeAgent).
func TestMain(m *testing.M) {
	testutil.RunFakeAgentIfRequested()
	os.Exit(m.Run())
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo creates a git repository with one commit and a Hilvan project in it.
func newRepo(t *testing.T, setupHook ...string) (*store.ProjectStore, string) {
	t.Helper()
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	if out, err := exec.Command("git", "init", "-q", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	git(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")
	s := store.NewProjectStore(repo)
	if _, err := s.Init("repo", "main", setupHook); err != nil {
		t.Fatal(err)
	}
	return s, tmp
}

func fakeProfile(tmp, id, app string, accepts ...string) *domain.Profile {
	cfg := filepath.Join(tmp, id)
	if len(accepts) == 0 {
		accepts = []string{"public", "personal"}
	}
	p := &domain.Profile{ID: id, Label: id, App: app, Auth: domain.Auth{Mode: "subscription"}, ConfigDir: &cfg,
		Destination: domain.Destination{Launcher: "native", Executable: os.Args[0]}, Accepts: accepts}
	p.Normalize()
	return p
}

func host(scenario string) launchers.Host {
	h := CurrentHost()
	h.InsideWSL = false
	h.Env = testutil.FakeAgentEnv(process.CurrentEnv(), scenario)
	return h
}

func deps(scenario string) RunDeps { return RunDeps{Host: host(scenario)} }

func mustCreate(t *testing.T, s *store.ProjectStore, b Brief) domain.Task {
	t.Helper()
	task, err := CreateTask(s, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func errContains(t *testing.T, err error, pattern string) {
	t.Helper()
	if err == nil || !regexp.MustCompile(pattern).MatchString(err.Error()) {
		t.Fatalf("error %v does not match %q", err, pattern)
	}
}

// ── tasks (M2) ─────────────────────────────────────────────────────────────

func TestKeepsStateOutOfGit(t *testing.T) {
	s, _ := newRepo(t)
	if out := git(t, s.Root, "status", "--porcelain"); out != "" {
		t.Errorf("git status: %q (SR-016)", out)
	}
}

func TestWriteTaskRunsInWorktree(t *testing.T) {
	s, tmp := newRepo(t)
	c := criterio.New()
	task := mustCreate(t, s, Brief{Title: "Add hello file", Criteria: []string{"hello.txt exists"}, Classification: "personal", Mode: "write"})
	d := deps("write")
	d.Criterio = c
	r, err := RunTask(s, task.ID, fakeProfile(tmp, "fake", "claude-code"), d)
	if err != nil {
		t.Fatal(err)
	}
	if r.Run.State != "completed" || r.Task.State != domain.TaskInReview || deref(r.Task.Branch) != "hilvan/"+task.ID {
		t.Fatalf("run %+v task %+v", r.Run, r.Task)
	}
	if r.Run.Diff == nil || r.Run.Diff.FilesChanged != 1 || r.Run.Diff.Paths[0] != "hello.txt" || r.Run.Risk == nil || *r.Run.Risk < 2 {
		t.Fatalf("diff %+v risk %v", r.Run.Diff, r.Run.Risk)
	}
	if *r.Run.Usage.CostUSD != 0.0123 || r.Run.Limits[0].Name != "five_hour" || r.Run.Limits[0].Utilization != 0.4 {
		t.Fatalf("usage %+v limits %+v", r.Run.Usage, r.Run.Limits)
	}
	if strings.Contains(git(t, s.Root, "ls-tree", "--name-only", "main"), "hello.txt") || !strings.Contains(git(t, s.Root, "ls-tree", "--name-only", "hilvan/"+task.ID), "hello.txt") {
		t.Error("the change must live on the task branch only")
	}
	wt, _ := os.ReadFile(filepath.Join(deref(r.Task.Worktree), ".git"))
	if strings.Contains(string(wt), s.Root) {
		t.Errorf("worktree link is absolute (spike 001 needs relative paths): %s", wt)
	}
	file, _ := s.RunFile(task.ID, r.Run.ID)
	log, _ := os.ReadFile(file)
	if !strings.Contains(string(log), "[REDACTED:anthropic-key]") || strings.Contains(string(log), "sk-ant-api03") {
		t.Error("run log must be redacted (SR-013)")
	}
	if n := strings.Count(string(log), `"type":"result"`); n != 1 {
		t.Errorf("the result line is stored %d times; one entry per agent line", n)
	}
	if st, _ := os.Stat(file); runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Errorf("run log mode %o", st.Mode().Perm())
	}
	if v, _ := store.VerifyAudit(filepath.Join(s.Dir, "events.jsonl"), 0); !v.OK || v.Events < 3 {
		t.Errorf("audit %+v", v)
	}
}

func TestBlocksAndRefusesSilentSwitch(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "x", Classification: "personal", Mode: "read"})
	r, err := RunTask(s, task.ID, fakeProfile(tmp, "fake", "claude-code"), deps("limit"))
	if err != nil || r.Task.State != domain.TaskBlocked || r.Run.BlockedReason != "rate_limit" {
		t.Fatalf("limit: %+v %v", r.Run, err)
	}
	_, err = RunTask(s, task.ID, fakeProfile(tmp, "other", "claude-code"), deps("success"))
	errContains(t, err, "needs a handoff")
}

func TestRefusesClassification(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "x", Classification: "corporate", Mode: "read"})
	_, err := RunTask(s, task.ID, fakeProfile(tmp, "fake", "claude-code"), deps("success"))
	errContains(t, err, `does not accept "corporate"`)
}

func TestRecoversInterruptedRun(t *testing.T) {
	s, _ := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "x", Classification: "personal", Mode: "read"})
	task.State, task.ProfileID = domain.TaskRunning, ptr("fake")
	s.SaveTask(task, nil)
	os.WriteFile(s.LockFile(), []byte(`{"pid":999999,"taskId":"`+task.ID+`","runId":"run_01JABCDEFGHJKMNPQRSTVWXYZ0","startedAt":"2026-10-07T00:00:00.000Z"}`), 0o600)
	if id, err := RecoverStaleRun(s); err != nil || id != task.ID {
		t.Fatalf("recover: %q %v", id, err)
	}
	if after, _ := s.LoadTask(task.ID); after.State != domain.TaskUncertain {
		t.Errorf("state %s (AC-020)", after.State)
	}
	if s.ReadLock() != nil {
		t.Error("stale lock kept")
	}
}

func TestOneRunAtATime(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "x", Classification: "personal", Mode: "read"})
	s.AcquireLock(task.ID, "run_01JABCDEFGHJKMNPQRSTVWXYZ0")
	_, err := RunTask(s, task.ID, fakeProfile(tmp, "fake", "claude-code"), deps("success"))
	errContains(t, err, "another run is active")
	if ok, _ := CancelTask(s, "tsk_01JABCDEFGHJKMNPQRSTVWXYZ0"); ok {
		t.Error("cancelled a task that is not running")
	}
}

func TestDiscardRemovesWorktreeAndBranch(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "x", Classification: "personal", Mode: "write"})
	if _, err := RunTask(s, task.ID, fakeProfile(tmp, "fake", "claude-code"), deps("write")); err != nil {
		t.Fatal(err)
	}
	if _, err := DiscardTask(s, task.ID, nil, nil); err != nil {
		t.Fatal(err)
	}
	if out := git(t, s.Root, "branch", "--list", "hilvan/"+task.ID); out != "" {
		t.Errorf("branch kept: %q", out)
	}
	if after, _ := s.LoadTask(task.ID); after.State != domain.TaskDiscarded {
		t.Errorf("state %s", after.State)
	}
}

func TestBriefRedactionAndShortIDs(t *testing.T) {
	s, _ := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "deploy", Objective: "use key AKIAIOSFODNN7EXAMPLE", Classification: "personal", Mode: "read"})
	if b, _ := s.Brief(task.ID); !strings.Contains(b, "[REDACTED:aws-access-key]") {
		t.Errorf("brief %q", b)
	}
	if id, _ := s.ResolveTaskID(task.ID[len(task.ID)-6:]); id != task.ID {
		t.Error("short id")
	}
}

func TestSuggest(t *testing.T) {
	sg := Suggest(criterio.New(), Brief{Title: "Corregir bug del login", Objective: "Datos de clientes en Jira interno", Classification: "personal", Mode: "write"})
	if sg.TaskType.Value != "fix" || sg.Classification.Value != "corporate" {
		t.Errorf("suggestions %s %q", sg.TaskType.Value, sg.Classification.Value)
	}
}

func TestSetupHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses touch and false")
	}
	s, tmp := newRepo(t, "touch", "SETUP_DONE")
	p := fakeProfile(tmp, "fake", "claude-code")
	task := mustCreate(t, s, Brief{Title: "x", Classification: "personal", Mode: "write"})
	r, err := RunTask(s, task.ID, p, deps("success"))
	if err != nil || r.Run.Diff == nil || !contains(r.Run.Diff.Paths, "SETUP_DONE") {
		t.Fatalf("hook output not in the worktree: %+v %v", r.Run.Diff, err)
	}
	bad, _ := newRepo(t, "false")
	t2 := mustCreate(t, bad, Brief{Title: "y", Classification: "personal", Mode: "write"})
	_, err = RunTask(bad, t2.ID, p, deps("success"))
	errContains(t, err, `setup hook "false" failed`)
	if bad.ReadLock() != nil {
		t.Error("lock kept after a failing hook")
	}
	if after, _ := bad.LoadTask(t2.ID); after.Worktree == nil {
		t.Error("worktree not tracked, it could not be discarded")
	}
}

// ── profiles ───────────────────────────────────────────────────────────────

func regProfile(id, app string, dir *string) domain.Profile {
	p := domain.Profile{ID: id, Label: id, App: app, Auth: domain.Auth{Mode: "subscription"}, ConfigDir: dir,
		Destination: domain.Destination{Launcher: "native"}, Accepts: []string{"personal"}}
	p.Normalize()
	return p
}

func TestProfileRegistry(t *testing.T) {
	empty := domain.ProfilesFile{Version: 1}
	f, err := AddProfile(empty, regProfile("claude-corp", "claude-code", ptr("/h/a")))
	if err != nil {
		t.Fatal(err)
	}
	if f, err = AddProfile(f, regProfile("claude-personal", "claude-code", ptr("/h/b"))); err != nil || len(f.Profiles) != 2 {
		t.Fatal("two profiles of the same app (AC-001)")
	}
	if _, err := AddProfile(f, regProfile("x", "claude-code", ptr("/h/a/"))); err == nil {
		t.Error("shared config dir accepted (AC-017)")
	}
	def, _ := AddProfile(empty, regProfile("a", "claude-code", nil))
	_, err = AddProfile(def, regProfile("b", "claude-code", nil))
	errContains(t, err, "default login")
	if g, err := AddProfile(def, regProfile("c", "codex", nil)); err != nil || len(g.Profiles) != 2 {
		t.Error("one default login per app")
	}
	if g, _ := RemoveProfile(f, "claude-corp"); len(g.Profiles) != 1 {
		t.Error("remove")
	}
	if _, err := RemoveProfile(f, "zzz"); err == nil {
		t.Error("removed an unknown profile")
	}
	if got := ExpandHome("~/a", "/h"); got != filepath.Join("/h", "a") || ExpandHome("/abs", "/h") != "/abs" {
		t.Error("expand ~")
	}
}

func checkDeps(r process.Result, issues ...string) CheckDeps {
	return CheckDeps{
		Host:            launchers.Host{OS: "linux", Env: map[string]string{}},
		Run:             func(launchers.Invocation) (process.Result, error) { return r, nil },
		ConfigDirIssues: func(string) DirFindings { return DirFindings{Issues: issues} },
	}
}

func TestCheckProfile(t *testing.T) {
	p := regProfile("a", "claude-code", ptr("/h/a"))
	c := CheckProfile(&p, checkDeps(process.Result{Stdout: `{"loggedIn":true,"authMethod":"claude.ai","email":"a@x.io"}`}))
	if len(c.Issues) != 0 || c.Auth.Account != "a@x.io" {
		t.Errorf("logged in: %+v", c)
	}
	if c := CheckProfile(&p, checkDeps(process.Result{Code: -1, Err: "not_found"})); !strings.Contains(c.Issues[0], "not found") {
		t.Errorf("missing executable: %v", c.Issues)
	}
	if c := CheckProfile(&p, checkDeps(process.Result{Stdout: `{"loggedIn":false}`})); !strings.Contains(strings.Join(c.Issues, ","), "not logged in") {
		t.Errorf("logged out: %v", c.Issues)
	}
	cx := regProfile("c", "codex", ptr("/h/c"))
	if c := CheckProfile(&cx, checkDeps(process.Result{Stdout: "Logged in using ChatGPT"})); !contains(c.Warnings, "active account cannot be verified") {
		t.Errorf("codex: %v", c.Warnings)
	}
	k := regProfile("k", "claude-code", ptr("/h/k"))
	k.Auth = domain.Auth{Mode: "api-key", APIKeyEnv: "WORK_KEY"}
	if c := CheckProfile(&k, checkDeps(process.Result{Stdout: `{"loggedIn":true,"authMethod":"api_key"}`})); !strings.Contains(strings.Join(c.Issues, ","), "WORK_KEY") {
		t.Errorf("api key: %v", c.Issues)
	}
	auth := func(acc string) ProfileCheck {
		return ProfileCheck{App: "claude-code", ProfileID: acc[:1], Auth: struct2(acc)}
	}
	w := SharedAccountWarnings([]ProfileCheck{auth("a A@x.io"), auth("b a@x.io")})
	if len(w) != 1 || w[0] != "profiles a, b are logged into the same account" {
		t.Errorf("shared account: %v", w)
	}
}

func TestWSLDiscovery(t *testing.T) {
	inv := WSLDiscoveryInvocation("Ubuntu", "claude", launchers.Host{OS: "win32"})
	if inv.Command != "wsl.exe" || inv.Args[0] != "-d" || inv.Args[1] != "Ubuntu" || inv.Args[4] != "-ic" {
		t.Fatalf("invocation %+v", inv)
	}
	exe, path, err := ParseWSLDiscovery("Welcome banner\r\n/home/u/.nvm/versions/node/v24.21.0/bin/claude\r\nHILVAN_PATH=/home/u/.nvm/versions/node/v24.21.0/bin:/usr/bin\r\n")
	if err != nil || exe != "/home/u/.nvm/versions/node/v24.21.0/bin/claude" || path != "/home/u/.nvm/versions/node/v24.21.0/bin:/usr/bin" {
		t.Errorf("parse: %q %q %v", exe, path, err)
	}
	if _, _, err := ParseWSLDiscovery("HILVAN_PATH=/usr/bin\n"); err == nil {
		t.Error("missing CLI accepted")
	}
}

// ── tools and reserved routing (FR-021) ───────────────────────────────────

func routingProfiles() []domain.Profile {
	corp := regProfile("claude-corp", "claude-code", ptr("/h/corp"))
	corp.Accepts = []string{"public", "personal", "corporate"}
	corp.Tags, corp.Routing = []string{"corporate"}, "reserved"
	corp.Tools.Provides = []string{"jira", "intranet-search"}
	corp.Tools.Aliases = domain.Aliases{"intranet-search": {"portal de empleados"}}
	personal := regProfile("claude-personal", "claude-code", ptr("/h/personal"))
	personal.Accepts = []string{"public", "personal", "corporate"}
	return []domain.Profile{corp, personal}
}

func rec(task domain.Task, profiles []domain.Profile) Recommendation {
	return Recommend(policy.Merge(policy.Layers{}), task, profiles, criterio.New(), nil, 1)
}

func ids(r Recommendation) []string {
	var out []string
	for _, x := range r.Ranked {
		out = append(out, x.Profile.ID)
	}
	return out
}

func TestReservedRouting(t *testing.T) {
	ps := routingProfiles()
	r := rec(domain.Task{Title: "Refactorizar el parser", Classification: "personal"}, ps)
	if strings.Join(ids(r), ",") != "claude-personal" {
		t.Fatalf("ranked %v", ids(r))
	}
	var corpEx *Excluded
	for i := range r.Excluded {
		if r.Excluded[i].Profile.ID == "claude-corp" {
			corpEx = &r.Excluded[i]
		}
	}
	if corpEx == nil || corpEx.Evaluation.Exclusions[0].Source != "routing" || !strings.Contains(corpEx.Evaluation.Exclusions[0].Reason, "reserved for tasks that need jira, intranet-search") {
		t.Errorf("reserved exclusion %+v", corpEx)
	}
	r = rec(domain.Task{Title: "x", Classification: "personal", Requires: []string{"jira"}}, ps)
	if strings.Join(ids(r), ",") != "claude-corp" || !strings.Contains(r.Ranked[0].Why[0].Signal, "reservado: la tarea requiere jira") {
		t.Errorf("own tool: %v %+v", ids(r), r.Ranked)
	}
	ps[1].Accepts = []string{"public"}
	r = rec(domain.Task{Title: "x", Classification: "corporate"}, ps)
	if strings.Join(ids(r), ",") != "claude-corp" || !strings.Contains(r.Ranked[0].Why[0].Signal, "único perfil elegible") {
		t.Errorf("last resort: %v", ids(r))
	}
}

func TestToolsDiscovery(t *testing.T) {
	if ToolIDFromName("claude.ai Jira Cloud") != "claude-ai-jira-cloud" || ToolIDFromName("Búsqueda Intranet") != "busqueda-intranet" || ToolIDFromName("_x") != "x" {
		t.Errorf("tool ids %q %q", ToolIDFromName("claude.ai Jira Cloud"), ToolIDFromName("Búsqueda Intranet"))
	}
	corp := routingProfiles()[0]
	d := DiscoverTools(&corp, CheckDeps{
		Host: launchers.Host{OS: "linux", Env: map[string]string{}},
		Run: func(inv launchers.Invocation) (process.Result, error) {
			if strings.Join(inv.Args, " ") != "mcp list" || inv.Cwd == "" {
				t.Errorf("invocation %+v (must run outside the repository)", inv)
			}
			return process.Result{Stdout: "jira: https://x - ✔ Connected\nconfluence: https://y - ✔ Connected"}, nil
		},
	})
	if strings.Join(d.Missing, ",") != "confluence" || strings.Join(d.Stale, ",") != "intranet-search" {
		t.Errorf("discovery %+v", d)
	}
}

func TestUsageByProfile(t *testing.T) {
	run := func(id, state, at string, cost *float64, in int64, limits ...domain.LimitWindow) domain.RunSummary {
		return domain.RunSummary{ID: "run_01JABCDEFGHJKMNPQRSTVWXYZ0", ProfileID: id, State: state, StartedAt: at,
			Usage: &domain.Usage{Estimated: true, CostUSD: cost, InputTokens: &in}, Limits: limits}
	}
	tasks := []domain.Task{
		{ProfileID: ptr("a"), State: domain.TaskAccepted, Runs: []domain.RunSummary{
			run("a", "completed", "2026-10-07T01:00:00.000Z", ptr(0.1), 10),
			run("a", "blocked", "2026-10-07T02:00:00.000Z", ptr(0.1), 10, domain.LimitWindow{Name: "five_hour", Utilization: 1}),
		}},
		{ProfileID: ptr("b"), State: domain.TaskDiscarded, Runs: []domain.RunSummary{run("b", "completed", "2026-10-07T01:00:00.000Z", nil, 5)},
			Reviews: []domain.Review{{ProfileID: "a", Verdict: "approve", Usage: &domain.Usage{Estimated: true, CostUSD: ptr(0.05)}}}},
	}
	u := UsageByProfile(tasks)
	a, b := u[0], u[1]
	if a.ProfileID != "a" || a.Runs != 2 || a.CostRuns != 3 || a.InputTokens != 20 || a.Accepted != 1 || a.Reviews != 1 || a.States["blocked"] != 1 {
		t.Errorf("a %+v", a)
	}
	if d := a.CostUSD - 0.25; d > 1e-9 || d < -1e-9 || a.LastLimits.Windows[0].Utilization != 1 {
		t.Errorf("a cost %v limits %+v", a.CostUSD, a.LastLimits)
	}
	if b.ProfileID != "b" || b.CostRuns != 0 || b.Discarded != 1 {
		t.Errorf("b %+v", b)
	}
}

// struct2 builds a logged-in status from "<id> <account>".
func struct2(s string) adapters.AuthStatus {
	_, acc, _ := strings.Cut(s, " ")
	return adapters.AuthStatus{LoggedIn: true, Verifiable: true, Account: acc}
}
