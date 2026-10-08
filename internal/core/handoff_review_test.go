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
