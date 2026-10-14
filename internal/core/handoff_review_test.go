package core

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/store"
)

func noPolicy() domain.EffectivePolicy { return policy.Merge(policy.Layers{}) }

// ── hand-off (M4) ──────────────────────────────────────────────────────────

func handoffProfile(tmp, id, app string) *domain.Profile {
	p := fakeProfile(tmp, id, app, "public", "personal", "corporate")
	p.Tags = []string{"personal"}
	return p
}

func TestHandoffContinuesBlockedTask(t *testing.T) {
	s, tmp := newRepo(t)
	a, b := handoffProfile(tmp, "claude-a", "claude-code"), handoffProfile(tmp, "codex-b", "codex")
	task := mustCreate(t, s, Brief{Title: "Add hello file", Objective: "Create hello.txt", Classification: "personal", Mode: "write"})
	if _, err := RunTask(s, task.ID, a, deps("write")); err != nil {
		t.Fatal(err)
	}
	blocked, err := RunTask(s, task.ID, a, deps("limit"))
	if err != nil || blocked.Task.State != domain.TaskBlocked {
		t.Fatalf("limit: %v %+v", err, blocked.Task)
	}
	draft, err := DraftHandoff(s, task.ID, b, a, noPolicy(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, re := range []string{`## Objetivo`, `bloqueada por rate_limit`, `NO verificados`, `commit [0-9a-f]+ hilvan\(`, `hello\.txt`, `\[REDACTED:anthropic-key\]`} {
		if !regexp.MustCompile(re).MatchString(draft.Body) {
			t.Errorf("package lacks %s:\n%s", re, draft.Body)
		}
	}
	if !draft.Evaluation.Eligible || len(draft.Secrets) == 0 {
		t.Errorf("eligible %v secrets %d", draft.Evaluation.Eligible, len(draft.Secrets))
	}
	edited := strings.Replace(draft.Body, "_(añada aquí decisiones tomadas que el siguiente agente deba respetar)_", "Usar UTF-8", 1)
	task2, hash, file, err := ApproveHandoff(s, draft, edited)
	if err != nil {
		t.Fatal(err)
	}
	if hash != SHA256(edited) || deref(task2.ProfileID) != "codex-b" || task2.State != domain.TaskReady {
		t.Errorf("approved task %+v", task2)
	}
	stored, _ := os.ReadFile(file)
	if !strings.Contains(string(stored), "sha256: "+hash) || !strings.Contains(string(stored), "Usar UTF-8") {
		t.Error("stored package")
	}
	if st, _ := os.Stat(file); runtime.GOOS != "windows" && st.Mode().Perm() != 0o600 {
		t.Errorf("package mode %o", st.Mode().Perm())
	}
	pkg, _ := s.Handoff(task.ID, draft.ID)
	brief, _ := s.Brief(task.ID)
	prompt := BuildPrompt(task2, brief, "/wt", pkg)
	if !strings.Contains(prompt, "Paquete de continuidad") || !strings.Contains(prompt, `Treat "assumptions" and agent claims as unverified`) {
		t.Error("prompt lacks the package")
	}
	next, err := RunTask(s, task.ID, b, deps("success"))
	if err != nil || next.Run.ProfileID != "codex-b" {
		t.Fatalf("continue: %v %+v", err, next.Run)
	}
	audit, _ := os.ReadFile(filepath.Join(s.Dir, "events.jsonl"))
	if !strings.Contains(string(audit), `"handoff.approved"`) || !strings.Contains(string(audit), hash) || strings.Contains(string(audit), "Usar UTF-8") {
		t.Error("audit keeps hashes, not content (SR-018)")
	}
	if v, _ := store.VerifyAudit(filepath.Join(s.Dir, "events.jsonl"), 0); !v.OK {
		t.Errorf("audit %+v", v)
	}
}

func TestCancelledHandoffSendsNothing(t *testing.T) {
	s, tmp := newRepo(t)
	a, b := handoffProfile(tmp, "claude-a", "claude-code"), handoffProfile(tmp, "codex-b", "codex")
	task := mustCreate(t, s, Brief{Title: "x", Classification: "personal", Mode: "read"})
	RunTask(s, task.ID, a, deps("limit"))
	draft, _ := DraftHandoff(s, task.ID, b, a, noPolicy(), nil)
	CancelHandoff(s, draft)
	after, _ := s.LoadTask(task.ID)
	if deref(after.ProfileID) != "claude-a" || len(after.Handoffs) != 0 {
		t.Errorf("after cancel %+v (AC-004)", after)
	}
	_, err := RunTask(s, task.ID, b, deps("success"))
	errContains(t, err, "needs a handoff")
}

func TestHandoffRefusals(t *testing.T) {
	s, tmp := newRepo(t)
	a := handoffProfile(tmp, "claude-a", "claude-code")
	corpOnly := fakeProfile(tmp, "corp", "claude-code", "corporate")
	task := mustCreate(t, s, Brief{Title: "x", Classification: "personal", Mode: "read"})
	RunTask(s, task.ID, a, deps("success"))
	draft, _ := DraftHandoff(s, task.ID, corpOnly, a, noPolicy(), nil)
	if draft.Evaluation.Eligible {
		t.Error("ineligible target")
	}
	_, _, _, err := ApproveHandoff(s, draft, draft.Body)
	errContains(t, err, "not eligible")
	ok, _ := DraftHandoff(s, task.ID, handoffProfile(tmp, "codex-b", "codex"), a, noPolicy(), nil)
	_, _, _, err = ApproveHandoff(s, ok, ok.Body+"\nAKIAIOSFODNN7EXAMPLE")
	errContains(t, err, "possible secrets")
}

func TestEnforcedRuleBlocksHandoff(t *testing.T) {
	s, tmp := newRepo(t)
	a := handoffProfile(tmp, "claude-corp", "claude-code")
	a.Tags = []string{"corporate"}
	b := handoffProfile(tmp, "claude-personal", "claude-code")
	task := mustCreate(t, s, Brief{Title: "x", Classification: "corporate", Mode: "read"})
	if _, err := RunTask(s, task.ID, a, deps("success")); err != nil {
		t.Fatal(err)
	}
	managed := policy.Merge(policy.Layers{Managed: &domain.PolicyFile{Version: 1, Rules: []domain.Rule{{
		ID: "no-corp-out", From: domain.Selector{Tags: []string{"corporate"}}, To: domain.Selector{Tags: []string{"personal"}},
		Effect: "deny", Enforced: true, Reason: "company policy",
	}}}})
	draft, _ := DraftHandoff(s, task.ID, b, a, managed, nil)
	if draft.Evaluation.Eligible || !draft.Evaluation.Exclusions[0].Enforced || draft.Evaluation.Exclusions[0].Reason != "company policy" {
		t.Errorf("AC-014: %+v", draft.Evaluation)
	}
}

// ── review and accept (M5) ─────────────────────────────────────────────────

func TestParseVerdict(t *testing.T) {
	if ParseVerdict("ok\nVERDICT: approve") != "approve" || ParseVerdict("VERDICT: Changes_Requested") != "changes_requested" || ParseVerdict("looks fine") != "unknown" {
		t.Error("verdicts")
	}
}

func writtenTask(t *testing.T) (*store.ProjectStore, string, string) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "Add hello", Classification: "personal", Mode: "write", TaskType: "implement"})
	if _, err := RunTask(s, task.ID, fakeProfile(tmp, "claude-a", "claude-code"), deps("write")); err != nil {
		t.Fatal(err)
	}
	return s, tmp, task.ID
}

func TestAcceptRequiresReview(t *testing.T) {
	s, _, id := writtenTask(t)
	_, err := AcceptTask(s, id, nil, nil)
	errContains(t, err, "review the current diff first")
	if _, diff, _, _ := ReviewDiff(s, id, nil); !strings.Contains(diff, "hello from the agent") {
		t.Errorf("diff %q", diff)
	}
}

func TestAcceptMergesAndTeaches(t *testing.T) {
	s, _, id := writtenTask(t)
	ReviewDiff(s, id, nil)
	c := criterio.New()
	task, err := AcceptTask(s, id, c, nil)
	if err != nil || task.State != domain.TaskAccepted {
		t.Fatalf("accept: %v %+v", err, task)
	}
	if !strings.Contains(git(t, s.Root, "ls-tree", "--name-only", "main"), "hello.txt") || git(t, s.Root, "branch", "--list", "hilvan/"+id) != "" {
		t.Error("merge and cleanup (AC-010)")
	}
	st := c.Stats().Profiles
	if len(st) != 1 || st[0].Key != "claude-a|implement" || st[0].N != 1 || st[0].Mean < 0.66 || st[0].Mean > 0.67 {
		t.Errorf("Criterio learned %+v (AC-021)", st)
	}
}

func TestAcceptRefusesDirtyOrOtherCheckout(t *testing.T) {
	s, _, id := writtenTask(t)
	ReviewDiff(s, id, nil)
	os.WriteFile(filepath.Join(s.Root, "tracked.txt"), []byte("x"), 0o644)
	git(t, s.Root, "add", "tracked.txt")
	_, err := AcceptTask(s, id, nil, nil)
	errContains(t, err, "uncommitted changes")
	git(t, s.Root, "reset", "-q", "tracked.txt")
	git(t, s.Root, "checkout", "-q", "-b", "other")
	_, err = AcceptTask(s, id, nil, nil)
	errContains(t, err, "check out main")
}

func TestCrossReview(t *testing.T) {
	s, tmp, id := writtenTask(t)
	_, err := CrossReview(s, id, fakeProfile(tmp, "claude-a", "claude-code"), deps("review"))
	errContains(t, err, "different profile")
	r, err := CrossReview(s, id, fakeProfile(tmp, "claude-b", "claude-code"), deps("review"))
	if err != nil || r.Verdict != "changes_requested" {
		t.Fatalf("review: %v %+v", err, r)
	}
	task, _ := s.LoadTask(id)
	if len(task.Reviews) != 1 || task.Reviews[0].ProfileID != "claude-b" || deref(task.ProfileID) != "claude-a" || task.State != domain.TaskInReview {
		t.Errorf("reviewing never switches identity (AC-018): %+v", task)
	}
}

func TestDiscardTeachesRejection(t *testing.T) {
	s, _, id := writtenTask(t)
	c := criterio.New()
	if _, err := DiscardTask(s, id, nil, func(task domain.Task) { LearnOutcome(task, false, c) }); err != nil {
		t.Fatal(err)
	}
	if st := c.Stats().Profiles; len(st) != 1 || st[0].Key != "claude-a|implement" || st[0].N != 1 || st[0].Mean > 0.34 {
		t.Errorf("rejection %+v", st)
	}
}

// ── approval gate in RunTask (AC-003, AC-004, AC-014) ─────────────────────

var corpToPersonal = &domain.PolicyFile{Version: 1, Rules: []domain.Rule{{
	ID: "corp-to-personal", From: domain.Selector{Classification: []string{"corporate", "confidential"}},
	To: domain.Selector{Tags: []string{"personal"}}, Effect: "require_approval",
	Reason: "corporate data to a personal account needs approval",
}}}

func personalProfile(tmp string) *domain.Profile {
	p := handoffProfile(tmp, "claude-personal", "claude-code")
	p.Accepts = []string{"public", "personal", "corporate"}
	return p
}

func TestApprovalShowsBriefAndDeclineSendsNothing(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "Informe trimestral", Objective: "datos internos", Classification: "corporate", Mode: "read"})
	pol := policy.Merge(policy.Layers{User: corpToPersonal})
	seen := ""
	d := deps("success")
	d.Policy = &pol
	d.Approve = func(_ policy.Evaluation, brief string) bool { seen = brief; return false }
	_, err := RunTask(s, task.ID, personalProfile(tmp), d)
	errContains(t, err, "nothing was sent")
	if !strings.Contains(seen, "Informe trimestral") {
		t.Errorf("the approver did not see the exact brief: %q", seen)
	}
	if after, _ := s.LoadTask(task.ID); len(after.Runs) != 0 {
		t.Error("a declined approval still ran the agent")
	}
}

func TestApprovedRunGoesAhead(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "x", Classification: "corporate", Mode: "read"})
	pol := policy.Merge(policy.Layers{User: corpToPersonal})
	d := deps("success")
	d.Policy = &pol
	d.Approve = func(policy.Evaluation, string) bool { return true }
	r, err := RunTask(s, task.ID, personalProfile(tmp), d)
	if err != nil || r.Run.State != domain.RunCompleted || !contains(r.Task.ApprovedProfiles, "claude-personal") {
		t.Fatalf("approved run: %v %+v", err, r.Task)
	}
}

func TestEnforcedDenyCannotBeApproved(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "x", Classification: "corporate", Mode: "read"})
	deny := corpToPersonal.Rules[0]
	deny.Effect, deny.Enforced = "deny", true
	pol := policy.Merge(policy.Layers{Managed: &domain.PolicyFile{Version: 1, Rules: []domain.Rule{deny}}})
	d := deps("success")
	d.Policy = &pol
	d.Approve = func(policy.Evaluation, string) bool { return true }
	_, err := RunTask(s, task.ID, personalProfile(tmp), d)
	errContains(t, err, "cannot be approved")
}

// ── required tools (AC-024) ────────────────────────────────────────────────

func TestRequiredToolsCatalogAndLearning(t *testing.T) {
	ps := routingProfiles()
	catalog := criterio.BuildToolCatalog(ps)
	aliases := map[string][]string{}
	for _, e := range catalog {
		aliases[e.Tool] = e.Aliases
	}
	for _, w := range []string{"jira", "ticket", "incidencia"} {
		if !contains(aliases["jira"], w) {
			t.Errorf("jira catalog lacks %q: %v", w, aliases["jira"])
		}
	}
	for _, w := range []string{"intranet", "search", "portal de empleados"} {
		if !contains(aliases["intranet-search"], w) {
			t.Errorf("intranet-search catalog lacks %q", w)
		}
	}
	for title, want := range map[string]string{
		"Revisar las incidencias del sprint en Jira":      "jira",
		"Buscar en la intranet la política de vacaciones": "intranet-search",
		"Consultar el portal de empleados":                "intranet-search",
		"Refactorizar el parser de eventos":               "",
	} {
		if got := strings.Join(criterio.New().RequiredTools(criterio.TaskSignals{Title: title}, catalog).Value, ","); got != want {
			t.Errorf("%q → %q, want %q", title, got, want)
		}
	}
	c := criterio.New()
	if v := c.RequiredTools(criterio.TaskSignals{Title: "Actualizar el estado del caso de soporte 4521"}, catalog).Value; len(v) != 0 {
		t.Fatalf("before learning: %v", v)
	}
	for _, n := range []string{"4521", "3310", "9087", "1200", "5555", "7001", "8123", "2222"} {
		c.LearnRequiredTools(criterio.TaskSignals{Title: "Actualizar el estado del caso de soporte " + n}, catalog, []string{"jira"})
	}
	if v := c.RequiredTools(criterio.TaskSignals{Title: "Cerrar el caso de soporte 6060"}, catalog).Value; !contains(v, "jira") {
		t.Errorf("did not learn from confirmations: %v", v)
	}
	if c.Stats().Examples[criterio.QTools] == 0 {
		t.Error("tool examples not counted")
	}
}

// D-028: the transcript of a cross review stays in its local run log; the task keeps a summary.
func TestReviewKeepsTranscriptInRunLogOnly(t *testing.T) {
	s, tmp, id := writtenTask(t)
	if _, err := CrossReview(s, id, fakeProfile(tmp, "claude-b", "claude-code"), deps("review")); err != nil {
		t.Fatal(err)
	}
	task, _ := s.LoadTask(id)
	if len(task.Reviews) != 1 || task.Reviews[0].Summary == "" || len(task.Reviews[0].Summary) > 4000 {
		t.Fatalf("summary not kept on the task: %+v", task.Reviews)
	}
	logPath, err := s.RunFile(id, task.Reviews[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(logPath); err != nil || len(data) == 0 {
		t.Errorf("transcript missing from the run log: %v", err)
	}
}

// D-030: the model is fixed on the first run and reused when the profile's model changes later.
func TestModelIsPinnedOnFirstRun(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "Pin model", Classification: "personal", Mode: "read", TaskType: "docs"})
	p := fakeProfile(tmp, "fake", "claude-code")
	m1 := "model-one"
	p.Model = &m1
	if _, err := RunTask(s, task.ID, p, deps("success")); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.LoadTask(task.ID)
	if saved.Model == nil || *saved.Model != "model-one" {
		t.Fatalf("model not pinned: %+v", saved.Model)
	}
	m2 := "model-two"
	p.Model = &m2
	if _, err := RunTask(s, task.ID, p, deps("success")); err != nil {
		t.Fatal(err)
	}
	again, _ := s.LoadTask(task.ID)
	if again.Model == nil || *again.Model != "model-one" {
		t.Errorf("resume switched model to %v (D-030)", again.Model)
	}
}

// D-027: a profile that is busy in another project refuses a new run with a clear message.
func TestBusyProfileRefusesRun(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "Busy", Classification: "personal", Mode: "read", TaskType: "docs"})
	locks := filepath.Join(tmp, "locks")
	if err := store.AcquireProfileLock(locks, "claude-a", store.Lock{PID: os.Getpid(), TaskID: "tsk_other", RunID: "run_other"}); err != nil {
		t.Fatal(err)
	}
	d := deps("success")
	d.ProfileLocks = locks
	_, err := RunTask(s, task.ID, fakeProfile(tmp, "claude-a", "claude-code"), d)
	if err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("busy profile ran: %v", err)
	}
	if after, _ := s.LoadTask(task.ID); after.State == domain.TaskRunning {
		t.Error("a refused run left the task running")
	}
}

// A handoff clears the model pinned by the previous profile: the model belongs to that profile
// (D-030), so the continuing profile must use its own.
func TestHandoffClearsPinnedModel(t *testing.T) {
	s, tmp := newRepo(t)
	task := mustCreate(t, s, Brief{Title: "Modelo", Classification: "personal", Mode: "read", TaskType: "docs"})
	a := fakeProfile(tmp, "claude-a", "claude-code")
	m := "claude-haiku-5-5"
	a.Model = &m
	if _, err := RunTask(s, task.ID, a, deps("success")); err != nil {
		t.Fatal(err)
	}
	if saved, _ := s.LoadTask(task.ID); saved.Model == nil {
		t.Fatal("model not pinned on the first run")
	}
	cur, _ := s.LoadTask(task.ID)
	draft, err := DraftHandoff(s, cur.ID, fakeProfile(tmp, "codex-b", "codex"), a, noPolicy(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ApproveHandoff(s, draft, draft.Body); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.LoadTask(task.ID); after.Model != nil {
		t.Errorf("model %q survived the handoff to another application", *after.Model)
	}
}
