package core

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/gitops"
	"github.com/ferchosur/hilvan/internal/jsfmt"
	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/security"
	"github.com/ferchosur/hilvan/internal/store"
)

// M5 — review, cross review and acceptance (FR-015, AC-010, AC-018, AC-021, SR-024).

const maxDiff = 2 * 1024 * 1024

func jsNum(f float64) string { return jsfmt.Number(f) }

func headOf(s *store.ProjectStore, branch string, git gitops.Git) (string, error) {
	r, err := git([]string{"rev-parse", "--verify", branch + "^{commit}"}, s.Root)
	if err != nil {
		return "", err
	}
	if r.Code != 0 {
		return "", &gitops.GitError{Msg: "branch " + branch + " not found"}
	}
	return strings.TrimSpace(r.Stdout), nil
}

// ReviewDiff returns the task's diff against its base, bounded in size, and marks the current
// head as reviewed.
func ReviewDiff(s *store.ProjectStore, taskID string, git gitops.Git) (t domain.Task, diff string, truncated bool, err error) {
	if git == nil {
		git = gitops.DefaultGit
	}
	if t, err = s.LoadTask(taskID); err != nil || t.Branch == nil || t.BaseBranch == nil {
		return t, "", false, err
	}
	r, err := git([]string{"diff", "--no-color", "--no-ext-diff", *t.BaseBranch + "..." + *t.Branch}, s.Root)
	if err != nil {
		return t, "", false, err
	}
	if r.Code != 0 {
		return t, "", false, &gitops.GitError{Msg: "git diff failed: " + strings.TrimSpace(r.Stderr)}
	}
	head, err := headOf(s, *t.Branch, git)
	if err != nil {
		return t, "", false, err
	}
	t.ReviewedHead = &head
	if t, err = s.SaveTask(t, nil); err != nil {
		return t, "", false, err
	}
	if err = s.Audit(store.AuditEvent{Type: "review.viewed", TaskID: taskID, Data: map[string]any{"head": head}}); err != nil {
		return t, "", false, err
	}
	diff = r.Stdout
	if len(diff) > maxDiff {
		return t, diff[:maxDiff], true, nil
	}
	return t, diff, false, nil
}

// ReviewPrompt asks another profile for a read-only review ending with a verdict line.
func ReviewPrompt(t domain.Task, brief string) string {
	inspect := "There is no code change; evaluate the previous result described below against the brief."
	if t.Branch != nil && t.BaseBranch != nil {
		inspect = "Inspect the changes with `git diff " + *t.BaseBranch + "..." + *t.Branch + "` and `git log " + *t.BaseBranch + ".." + *t.Branch + "`."
	}
	lines := []string{
		"You are an independent reviewer. This is a READ-ONLY task: do not modify any file.",
		inspect,
		"Check every acceptance criterion of the brief, look for bugs, security issues and unverified claims.",
		"Answer with a short list of findings (severity: high/medium/low), then a final line exactly:",
		"VERDICT: approve   or   VERDICT: changes_requested",
		"",
		brief,
	}
	if n := len(t.Runs); n > 0 && t.Runs[n-1].Summary != "" {
		lines = append(lines, "", "Previous agent summary (unverified):", t.Runs[n-1].Summary)
	}
	return strings.Join(lines, "\n")
}

var verdictLine = regexp.MustCompile(`(?i)VERDICT:\s*(approve|changes_requested)`)

// ParseVerdict reads approve, changes_requested or unknown.
func ParseVerdict(text string) string {
	if m := verdictLine.FindStringSubmatch(text); m != nil {
		return strings.ToLower(m[1])
	}
	return "unknown"
}

// CrossReviewResult is the attached review.
type CrossReviewResult struct {
	Task    domain.Task
	Verdict string
	Text    string
	File    string
}

// CrossReview runs a read-only review with another profile and attaches it to the task
// (AC-018). Reviewing never switches the task's identity.
func CrossReview(s *store.ProjectStore, taskID string, reviewer *domain.Profile, deps RunDeps) (CrossReviewResult, error) {
	t, err := s.LoadTask(taskID)
	if err != nil {
		return CrossReviewResult{}, err
	}
	switch {
	case len(t.Runs) == 0:
		return CrossReviewResult{}, taskError("nothing to review yet: run the task first")
	case t.State == domain.TaskRunning:
		return CrossReviewResult{}, taskError("the task is running")
	case t.ProfileID != nil && reviewer.ID == *t.ProfileID:
		return CrossReviewResult{}, taskError("a cross review needs a different profile")
	}
	ev := policy.Evaluate(deps.policy(), policy.TaskFacts{Classification: t.Classification, Requires: t.Requires}, reviewer, nil)
	if !ev.Eligible {
		return CrossReviewResult{}, taskError("profile %s is not eligible to review: %s", reviewer.ID, reasons(ev))
	}
	if err := approveIfNeeded(s, &t, reviewer, ev, deps, nil); err != nil {
		return CrossReviewResult{}, err
	}
	brief, err := s.Brief(taskID)
	if err != nil {
		return CrossReviewResult{}, err
	}

	runID := domain.NewRunID()
	if err := s.AcquireLock(taskID, runID); err != nil {
		return CrossReviewResult{}, err
	}
	defer s.ReleaseLock()
	cwd := s.Root
	if t.Worktree != nil {
		cwd = *t.Worktree
	}
	destCwd, err := launchers.DestinationPath(reviewer.Destination, deps.Host, cwd)
	if err != nil {
		return CrossReviewResult{}, err
	}
	adapter, _ := adapters.For(reviewer.App)
	inv, err := launchers.BuildInvocation(reviewer.Destination, adapter.RunCommand(reviewer, adapters.RunSpec{Cwd: destCwd, Mode: "read"}, deps.Host.Env), deps.Host)
	if err != nil {
		return CrossReviewResult{}, err
	}
	if err := s.Audit(store.AuditEvent{Type: "review.started", TaskID: taskID, RunID: runID, ProfileID: reviewer.ID}); err != nil {
		return CrossReviewResult{}, err
	}
	var messages []string
	h, err := adapters.StartRun(inv, ReviewPrompt(t, brief), adapter.NewParser(), func(e adapters.Event, _ string) {
		if e.Type == adapters.EvMessage {
			messages = append(messages, e.Text)
		}
		if deps.OnEvent != nil {
			deps.OnEvent(e)
		}
	}, deps.RunOptions)
	if err != nil {
		return CrossReviewResult{}, err
	}
	if deps.OnStart != nil {
		deps.OnStart(h)
	}
	out := h.Wait().Outcome
	raw := strings.Join(messages, "\n\n")
	if out.ResultText != nil && *out.ResultText != "" {
		raw = *out.ResultText
	}
	text, _ := security.RedactSecrets(raw)
	verdict := "unknown"
	if out.State == domain.RunCompleted {
		verdict = ParseVerdict(text)
	}
	file, err := s.ReviewFile(taskID, runID)
	if err != nil {
		return CrossReviewResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return CrossReviewResult{}, err
	}
	body := "# Revisión cruzada por " + reviewer.ID + "\n\n- Estado: " + out.State + "\n- Veredicto: " + verdict + "\n\n" + text + "\n"
	if err := security.WriteFileAtomic(file, []byte(body), 0o600); err != nil {
		return CrossReviewResult{}, err
	}
	if t, err = s.LoadTask(taskID); err != nil {
		return CrossReviewResult{}, err
	}
	t.Reviews = append(t.Reviews, domain.Review{ID: runID, ProfileID: reviewer.ID, Verdict: verdict, Usage: usageOf(out), At: store.Now()})
	if t, err = s.SaveTask(t, nil); err != nil {
		return CrossReviewResult{}, err
	}
	err = s.Audit(store.AuditEvent{Type: "review.attached", TaskID: taskID, RunID: runID, ProfileID: reviewer.ID,
		Data: map[string]any{"verdict": verdict, "state": out.State}})
	return CrossReviewResult{Task: t, Verdict: verdict, Text: text, File: file}, err
}

// AcceptTask accepts a task in review (AC-010, AC-021): write tasks are merged into the base
// branch only after the current diff was shown, from a clean checkout of the base branch; the
// worktree is then removed and Criterio learns.
func AcceptTask(s *store.ProjectStore, taskID string, c *criterio.Criterio, git gitops.Git) (domain.Task, error) {
	if git == nil {
		git = gitops.DefaultGit
	}
	t, err := s.LoadTask(taskID)
	if err != nil {
		return t, err
	}
	if t.State != domain.TaskInReview {
		return t, taskError("only tasks in review can be accepted (task is %s)", t.State)
	}
	if t.Mode == "write" && t.Branch != nil && t.BaseBranch != nil {
		head, err := headOf(s, *t.Branch, git)
		if err != nil {
			return t, err
		}
		if t.ReviewedHead == nil || *t.ReviewedHead != head {
			return t, taskError("review the current diff first: hilvan review <task> (SR-024)")
		}
		current, _ := git([]string{"rev-parse", "--abbrev-ref", "HEAD"}, s.Root)
		if branch := strings.TrimSpace(current.Stdout); branch != *t.BaseBranch {
			return t, taskError("check out %s in %s before accepting (now on %s)", *t.BaseBranch, s.Root, branch)
		}
		if dirty, _ := git([]string{"status", "--porcelain", "--untracked-files=no"}, s.Root); strings.TrimSpace(dirty.Stdout) != "" {
			return t, taskError("the main checkout has uncommitted changes; commit or stash them first")
		}
		args := append(gitops.FallbackIdentity(s.Root, git), "merge", "--no-ff", "--no-verify", "-m", "hilvan: "+t.Title+" ("+t.ID+")", "--", *t.Branch)
		merge, err := git(args, s.Root)
		if err != nil {
			return t, err
		}
		if merge.Code != 0 {
			git([]string{"merge", "--abort"}, s.Root)
			line, _, _ := strings.Cut(strings.TrimSpace(merge.Stderr), "\n")
			return t, taskError("merge failed (conflicts?): %s", line)
		}
		if err := gitops.RemoveWorktree(s.Root, t.ID, git); err != nil {
			return t, err
		}
	}
	t.State, t.Worktree, t.Branch = domain.TaskAccepted, nil, nil
	if t, err = s.SaveTask(t, nil); err != nil {
		return t, err
	}
	LearnOutcome(t, true, c)
	ev := store.AuditEvent{Type: "task.accepted", TaskID: taskID}
	if t.ProfileID != nil {
		ev.ProfileID = *t.ProfileID
	}
	return t, s.Audit(ev)
}

// LearnOutcome feeds the human decision back into Criterio (profile ranking and acceptance).
func LearnOutcome(t domain.Task, accepted bool, c *criterio.Criterio) {
	if c == nil || t.ProfileID == nil {
		return
	}
	typ := t.TaskType
	if typ == "" {
		typ = "implement"
	}
	c.RecordOutcome(*t.ProfileID, typ, accepted)
	if n := len(t.Runs); n > 0 {
		last := t.Runs[n-1]
		state := last.State
		if state == domain.RunRunning {
			state = domain.RunUncertain
		}
		cc := t.CriteriaCount
		rs := criterio.RunSignals{State: state, ResultText: last.Summary, CriteriaCount: &cc}
		if last.Diff != nil {
			rs.Diff = ptr(diffSignals(*last.Diff))
		}
		c.LearnAcceptance(rs, accepted)
	}
}
