// Package core holds Hilvan's use cases: tasks and runs, profiles, recommendation, hand-offs,
// reviews and usage. Presentation lives in the CLI.
package core

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/gitops"
	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/process"
	"github.com/ferchosur/hilvan/internal/security"
	"github.com/ferchosur/hilvan/internal/store"
)

// TaskError is a user-facing refusal of a task operation.
type TaskError struct{ Msg string }

func (e *TaskError) Error() string { return e.Msg }

func taskError(format string, args ...any) error {
	return &TaskError{Msg: fmt.Sprintf(format, args...)}
}

// IsTaskError reports whether err is a TaskError.
func IsTaskError(err error) bool {
	var t *TaskError
	return errors.As(err, &t)
}

// Brief is what the user asks for.
type Brief struct {
	Title          string
	Objective      string
	Criteria       []string
	Constraints    []string
	Classification string
	Mode           string // write | read
	Requires       []string
	TaskType       string
	Complexity     float64 // 0 = not set
}

// RenderBrief writes brief.md.
func RenderBrief(b Brief) string {
	sec := func(title string, items []string) string {
		if len(items) == 0 {
			return ""
		}
		lines := make([]string, len(items))
		for i, it := range items {
			lines[i] = "- " + it
		}
		return "\n## " + title + "\n\n" + strings.Join(lines, "\n") + "\n"
	}
	out := "# " + b.Title + "\n\n" + "- Clasificación: " + b.Classification + "\n- Modo: " + b.Mode + "\n"
	if b.Objective != "" {
		out += "\n## Objetivo\n\n" + b.Objective + "\n"
	}
	return out + sec("Criterios de aceptación", b.Criteria) + sec("Restricciones", b.Constraints)
}

// Suggestions are Criterio's proposals for a brief (FR-019, FR-020).
type Suggestions struct {
	TaskType       criterio.Decision[string]
	Complexity     criterio.Decision[float64]
	Classification criterio.Decision[string] // "" = keep the current one
}

func briefSignals(b Brief) criterio.TaskSignals {
	var parts []string
	for _, p := range append(append([]string{b.Objective}, b.Criteria...), b.Constraints...) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	n := len(b.Criteria)
	return criterio.TaskSignals{Title: b.Title, Text: strings.Join(parts, "\n"), CriteriaCount: &n}
}

// Suggest asks Criterio for the type, complexity and classification of a brief.
func Suggest(c *criterio.Criterio, b Brief) Suggestions {
	s := briefSignals(b)
	tt := c.TaskType(s)
	typ := b.TaskType
	if typ == "" {
		typ = tt.Value
	}
	return Suggestions{TaskType: tt, Complexity: c.Complexity(s, typ), Classification: c.SuggestClassification(s, b.Classification, nil)}
}

func ptr[T any](v T) *T { return &v }

// CreateTask stores a new task in the ready state. Secrets typed into the brief are redacted
// before it is stored or sent (SR-013).
func CreateTask(s *store.ProjectStore, b Brief, c *criterio.Criterio) (domain.Task, error) {
	project, err := s.Project()
	if err != nil {
		return domain.Task{}, err
	}
	now := store.Now()
	t := domain.Task{
		ID: domain.NewTaskID(), Title: b.Title, Classification: b.Classification, Mode: b.Mode,
		Requires: append([]string{}, b.Requires...), TaskType: b.TaskType, CriteriaCount: len(b.Criteria),
		BaseBranch: ptr(project.BaseBranch), State: domain.TaskReady, CreatedAt: now, UpdatedAt: now,
	}
	if b.Complexity != 0 {
		t.Complexity = ptr(b.Complexity)
	}
	brief, _ := security.RedactSecrets(RenderBrief(b))
	if t, err = s.SaveTask(t, &brief); err != nil {
		return t, err
	}
	if c != nil && b.TaskType != "" {
		c.LearnTaskType(criterio.TaskSignals{Title: b.Title, Text: b.Objective}, b.TaskType)
	}
	data := map[string]any{"classification": t.Classification, "mode": t.Mode}
	if t.TaskType != "" {
		data["taskType"] = t.TaskType
	}
	return t, s.Audit(store.AuditEvent{Type: "task.created", TaskID: t.ID, Data: data})
}

// BuildPrompt is what the agent receives on stdin. cwd is the destination's view.
func BuildPrompt(t domain.Task, brief, cwd, handoff string) string {
	mode := "Make the changes in this working directory; Hilvan will commit them to a review branch. Do not push."
	if t.Mode == "read" {
		mode = "This is a READ-ONLY task: do not modify files."
	}
	lines := []string{
		"You are working on a task coordinated by Hilvan.",
		"Task id: " + t.ID + ". Working directory: " + cwd + ".",
		mode,
		"When done, reply with a short summary of what you did, what you verified and anything left pending.",
		"",
		brief,
	}
	if handoff != "" {
		lines = append(lines, "", "---",
			"Another profile worked on this task before you. Its approved continuity package follows.",
			`Treat "assumptions" and agent claims as unverified: check them in the repository.`,
			"", handoff)
	}
	return strings.Join(lines, "\n")
}

// RunDeps are the collaborators of a run.
type RunDeps struct {
	Host launchers.Host
	// Policy is the effective layered policy; nil means built-in classifications and no rules.
	Policy *domain.EffectivePolicy
	// Approve is called when a rule requires explicit approval (FR-004), with the exact brief
	// that will be sent. Without it, such runs are refused.
	Approve func(ev policy.Evaluation, brief string) bool
	Git     gitops.Git
	// ProfileLocks is the directory of cross-project profile locks (D-027); empty disables them.
	ProfileLocks string
	Criterio     *criterio.Criterio
	RunOptions   adapters.RunOptions
	OnEvent      func(adapters.Event)
	// OnStart is called once the agent started, so the CLI can wire Ctrl+C to Cancel.
	OnStart func(*adapters.RunHandle)
}

func (d RunDeps) git() gitops.Git {
	if d.Git == nil {
		return gitops.DefaultGit
	}
	return d.Git
}

func (d RunDeps) policy() domain.EffectivePolicy {
	if d.Policy == nil {
		return policy.Merge(policy.Layers{})
	}
	return *d.Policy
}

// RunReport is the result of RunTask.
type RunReport struct {
	Task    domain.Task
	Run     domain.RunSummary
	Outcome adapters.Outcome
}

func stateFor(o adapters.Outcome) domain.TaskState {
	switch o.State {
	case domain.RunCompleted:
		return domain.TaskInReview
	case domain.RunBlocked:
		return domain.TaskBlocked
	case domain.RunFailed:
		return domain.TaskFailed
	case domain.RunCancelled:
		return domain.TaskReady
	}
	return domain.TaskUncertain
}

func reasons(ev policy.Evaluation) string {
	r := make([]string, len(ev.Exclusions))
	for i, e := range ev.Exclusions {
		r[i] = e.Reason
	}
	return strings.Join(r, "; ")
}

// approveIfNeeded asks for approval when a rule requires it and records the answer.
func approveIfNeeded(s *store.ProjectStore, t *domain.Task, p *domain.Profile, ev policy.Evaluation, deps RunDeps, audit map[string]any) error {
	if !ev.RequiresApproval || contains(t.ApprovedProfiles, p.ID) {
		return nil
	}
	brief, err := s.Brief(t.ID)
	if err != nil {
		return err
	}
	ok := deps.Approve != nil && deps.Approve(ev, brief)
	typ := "approval.denied"
	if ok {
		typ = "approval.granted"
	}
	if err := s.Audit(store.AuditEvent{Type: typ, TaskID: t.ID, ProfileID: p.ID, Data: audit}); err != nil {
		return err
	}
	if !ok {
		return taskError("approval required and not granted; nothing was sent")
	}
	t.ApprovedProfiles = append(t.ApprovedProfiles, p.ID)
	saved, err := s.SaveTask(*t, nil)
	*t = saved
	return err
}

// RunTask runs a task with a profile (FR-014, FR-016, FR-017, FR-018).
func RunTask(s *store.ProjectStore, taskID string, p *domain.Profile, deps RunDeps) (RunReport, error) {
	git := deps.git()
	if _, err := RecoverStaleRun(s); err != nil {
		return RunReport{}, err
	}
	t, err := s.LoadTask(taskID)
	if err != nil {
		return RunReport{}, err
	}
	switch t.State {
	case domain.TaskReady, domain.TaskFailed, domain.TaskBlocked, domain.TaskUncertain, domain.TaskInReview:
	default:
		return RunReport{}, taskError("task is %s; it cannot run now", t.State)
	}
	// FR-010/AC-007: never switch identity silently once a task has run with a profile.
	if t.ProfileID != nil && *t.ProfileID != p.ID && len(t.Runs) > 0 {
		return RunReport{}, taskError("task ran with %s; continuing with %s needs a handoff (M4)", *t.ProfileID, p.ID)
	}
	ev := policy.Evaluate(deps.policy(), policy.TaskFacts{Classification: t.Classification, Requires: t.Requires}, p, nil)
	sources := []any{}
	for _, e := range ev.Exclusions {
		sources = append(sources, e.Source)
	}
	if err := s.Audit(store.AuditEvent{Type: "policy.evaluated", TaskID: taskID, ProfileID: p.ID,
		Data: map[string]any{"eligible": ev.Eligible, "requiresApproval": ev.RequiresApproval, "exclusions": sources}}); err != nil {
		return RunReport{}, err
	}
	if !ev.Eligible {
		msg := fmt.Sprintf("profile %s is not eligible: %s", p.ID, reasons(ev))
		for _, e := range ev.Exclusions {
			if e.Enforced {
				msg += " (enforced by the managed policy; cannot be approved)"
				break
			}
		}
		return RunReport{}, taskError("%s", msg)
	}
	rules := []any{}
	for _, a := range ev.Approvals {
		rules = append(rules, a.RuleID)
	}
	if err := approveIfNeeded(s, &t, p, ev, deps, map[string]any{"rules": rules}); err != nil {
		return RunReport{}, err
	}

	runID := domain.NewRunID()
	if deps.ProfileLocks != "" {
		lock := store.Lock{PID: os.Getpid(), TaskID: taskID, RunID: runID, StartedAt: store.Now()}
		if err := store.AcquireProfileLock(deps.ProfileLocks, p.ID, lock); err != nil {
			return RunReport{}, taskError("%s", err)
		}
		defer store.ReleaseProfileLock(deps.ProfileLocks, p.ID, runID)
	}
	if err := s.AcquireLock(taskID, runID); err != nil {
		return RunReport{}, err
	}
	defer s.ReleaseLock()
	startedAt := store.Now()

	cwd := s.Root
	if t.Mode == "write" {
		if t.Worktree == nil {
			base := "HEAD"
			if t.BaseBranch != nil {
				base = *t.BaseBranch
			}
			path, branch, err := gitops.CreateWorktree(s.Root, t.ID, base, git)
			if err != nil {
				return RunReport{}, err
			}
			t.Worktree, t.Branch = &path, &branch
			// Persist the worktree first, so a failing hook never leaves an untracked worktree.
			if t, err = s.SaveTask(t, nil); err != nil {
				return RunReport{}, err
			}
			if err := RunSetupHook(s, t.ID, path); err != nil {
				return RunReport{}, err
			}
		}
		cwd = *t.Worktree
	}
	t.ProfileID, t.State = &p.ID, domain.TaskRunning
	if t, err = s.SaveTask(t, nil); err != nil {
		return RunReport{}, err
	}
	if err := s.Audit(store.AuditEvent{Type: "run.started", TaskID: taskID, RunID: runID, ProfileID: p.ID,
		Data: map[string]any{"destination": p.Destination.Launcher, "mode": t.Mode}}); err != nil {
		return RunReport{}, err
	}

	adapter, _ := adapters.For(p.App)
	// Sessions belong to an identity: only resume this profile's own session.
	lastSession := ""
	for i := len(t.Runs) - 1; i >= 0; i-- {
		if t.Runs[i].SessionID != "" && t.Runs[i].ProfileID == p.ID {
			lastSession = t.Runs[i].SessionID
			break
		}
	}
	// The agent sees the working directory from its own environment (C:\… vs /mnt/c/…).
	destCwd, err := launchers.DestinationPath(p.Destination, deps.Host, cwd)
	if err != nil {
		return RunReport{}, err
	}
	// D-030: the effective model is fixed the first time the task runs and reused on resume.
	if t.Model == nil && p.Model != nil {
		t.Model = ptr(*p.Model)
	}
	spec := adapters.RunSpec{Cwd: destCwd, Mode: t.Mode, ResumeSessionID: lastSession, Web: p.WebFor(t.Classification)}
	if t.Model != nil {
		spec.Model = *t.Model
	}
	cmd := adapter.RunCommand(p, spec, deps.Host.Env)
	inv, err := launchers.BuildInvocation(p.Destination, cmd, deps.Host)
	if err != nil {
		return RunReport{}, err
	}
	handoff := ""
	if lastSession == "" && len(t.Handoffs) > 0 {
		if handoff, err = s.Handoff(taskID, t.Handoffs[len(t.Handoffs)-1]); err != nil {
			return RunReport{}, err
		}
	}
	brief, err := s.Brief(taskID)
	if err != nil {
		return RunReport{}, err
	}
	runFile, err := s.RunFile(taskID, runID)
	if err != nil {
		return RunReport{}, err
	}
	if err := os.MkdirAll(filepath.Dir(runFile), 0o700); err != nil {
		return RunReport{}, err
	}
	logFile, err := os.OpenFile(runFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return RunReport{}, err
	}
	defer logFile.Close()
	opts := deps.RunOptions
	// SR-013: run logs are stored with secrets redacted, one entry per agent output line.
	opts.OnLine = func(raw string) {
		redacted, _ := security.RedactSecrets(raw)
		logFile.WriteString(redacted + "\n")
	}
	h, err := adapters.StartRun(inv, BuildPrompt(t, brief, destCwd, handoff), adapter.NewParser(), func(e adapters.Event, _ string) {
		if deps.OnEvent != nil {
			deps.OnEvent(e)
		}
	}, opts)
	if err != nil {
		return RunReport{}, err
	}
	if deps.OnStart != nil {
		deps.OnStart(h)
	}
	result := h.Wait()
	out := result.Outcome

	run := domain.RunSummary{ID: runID, ProfileID: p.ID, State: out.State, BlockedReason: out.BlockedReason,
		Usage: usageOf(out), Limits: out.Limits, StartedAt: startedAt}
	if out.SessionID != nil {
		run.SessionID = *out.SessionID
	}
	if out.Error != nil && *out.Error != "" {
		run.Error = truncate(*out.Error, 2000)
	}
	if out.ResultText != nil && *out.ResultText != "" {
		summary, _ := security.RedactSecrets(*out.ResultText)
		run.Summary = truncate(summary, RunSummaryLimit)
	}
	run.MaxSilenceSeconds = roundSeconds(result.MaxSilence)
	run.EndedAt = store.Now()

	if t.Mode == "write" && t.Worktree != nil && t.Branch != nil {
		if _, err := gitops.CommitWorktree(*t.Worktree, fmt.Sprintf("hilvan(%s): %s [%s]", t.ID, t.Title, runID), git); err != nil {
			return RunReport{}, err
		}
		base := "HEAD"
		if t.BaseBranch != nil {
			base = *t.BaseBranch
		}
		diff, err := gitops.DiffSummary(s.Root, base, *t.Branch, git)
		if err != nil {
			return RunReport{}, err
		}
		run.Diff = &diff
		if deps.Criterio != nil && diff.FilesChanged > 0 {
			run.Risk = ptr(deps.Criterio.DiffRisk(diffSignals(diff)).Value)
		}
	}
	if deps.Criterio != nil {
		cc := t.CriteriaCount
		rs := criterio.RunSignals{State: out.State, PermissionDenials: out.PermissionDenials, CriteriaCount: &cc}
		if out.ResultText != nil {
			rs.ResultText = *out.ResultText
		}
		if run.Diff != nil {
			rs.Diff = ptr(diffSignals(*run.Diff))
		}
		run.MeetsCriteria = ptr(deps.Criterio.MeetsCriteria(rs).P)
	}

	t.State = stateFor(out)
	t.Runs = append(t.Runs, run)
	if t, err = s.SaveTask(t, nil); err != nil {
		return RunReport{}, err
	}
	data := map[string]any{"state": out.State}
	if out.BlockedReason != "" {
		data["blockedReason"] = out.BlockedReason
	}
	if out.CostUSD != nil {
		data["costUsd"] = *out.CostUSD
	}
	if run.Diff != nil {
		data["files"] = run.Diff.FilesChanged
	}
	if err := s.Audit(store.AuditEvent{Type: "run.finished", TaskID: taskID, RunID: runID, ProfileID: p.ID, Data: data}); err != nil {
		return RunReport{}, err
	}
	return RunReport{Task: t, Run: run, Outcome: out}, nil
}

func diffSignals(d domain.DiffSummary) criterio.DiffSignals {
	return criterio.DiffSignals{FilesChanged: d.FilesChanged, LinesAdded: d.LinesAdded, LinesDeleted: d.LinesDeleted, Paths: d.Paths}
}

func usageOf(o adapters.Outcome) *domain.Usage {
	u := &domain.Usage{Estimated: true, CostUSD: o.CostUSD}
	if o.InputTokens != nil {
		u.InputTokens = ptr(int64(*o.InputTokens))
	}
	if o.OutputTokens != nil {
		u.OutputTokens = ptr(int64(*o.OutputTokens))
	}
	return u
}

// truncate cuts to n UTF-16 units, like String.prototype.slice in the TypeScript version.
func truncate(s string, n int) string {
	units := 0
	for i, r := range s {
		w := 1
		if r >= 0x10000 {
			w = 2
		}
		if units+w > n {
			return s[:i]
		}
		units += w
	}
	return s
}

// RunSetupHook runs the project's optional setup command once in a new worktree (FR-018):
// declared in .hilvan/project.json, without a shell and with a timeout.
func RunSetupHook(s *store.ProjectStore, taskID, worktree string) error {
	project, err := s.Project()
	if err != nil || len(project.SetupHook) == 0 {
		return err
	}
	name, args := project.SetupHook[0], project.SetupHook[1:]
	r, err := process.Run(name, args, process.Options{Dir: worktree, Timeout: 15 * time.Minute})
	if err != nil {
		return err
	}
	data := map[string]any{"command": strings.Join(project.SetupHook, " "), "exitCode": r.Code}
	if r.Err != "" {
		data["error"] = r.Err
	}
	if err := s.Audit(store.AuditEvent{Type: "worktree.setup", TaskID: taskID, Data: data}); err != nil {
		return err
	}
	if r.Code != 0 {
		detail := strings.TrimSpace(r.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(r.Stdout)
		}
		lines := strings.Split(detail, "\n")
		last := lines[len(lines)-1]
		if last == "" {
			last = r.Err
		}
		return taskError("setup hook %q failed: %s", strings.Join(project.SetupHook, " "), last)
	}
	return nil
}

// CancelTask asks the process running a task to stop (SIGINT; the runner escalates).
func CancelTask(s *store.ProjectStore, taskID string) (bool, error) {
	lock := s.ReadLock()
	if lock == nil || lock.TaskID != taskID || !store.ProcessAlive(lock.PID) {
		return false, nil
	}
	if err := interrupt(lock.PID); err != nil {
		return false, err
	}
	return true, s.Audit(store.AuditEvent{Type: "run.cancel_requested", TaskID: taskID, RunID: lock.RunID})
}

// RecoverStaleRun marks a run whose process died as uncertain, never re-run automatically
// (FR-017). It returns the task id it recovered, or "".
func RecoverStaleRun(s *store.ProjectStore) (string, error) {
	lock := s.ReadLock()
	if lock == nil || store.ProcessAlive(lock.PID) {
		return "", nil
	}
	defer s.ReleaseLock()
	t, err := s.LoadTask(lock.TaskID)
	if err == nil && t.State == domain.TaskRunning && domain.CanTransition(domain.TaskRunning, domain.TaskUncertain) {
		profile := "unknown"
		if t.ProfileID != nil {
			profile = *t.ProfileID
		}
		t.State = domain.TaskUncertain
		t.Runs = append(t.Runs, domain.RunSummary{ID: lock.RunID, ProfileID: profile, State: domain.RunUncertain,
			Error: "Hilvan stopped before the run finished", StartedAt: lock.StartedAt, EndedAt: store.Now()})
		if _, err := s.SaveTask(t, nil); err != nil {
			return "", err
		}
	}
	return lock.TaskID, s.Audit(store.AuditEvent{Type: "run.recovered", TaskID: lock.TaskID, RunID: lock.RunID})
}

// DiscardTask drops a task's worktree and branch without merging. onRejected is called with a
// task that was in review, e.g. to teach Criterio a rejection.
func DiscardTask(s *store.ProjectStore, taskID string, git gitops.Git, onRejected func(domain.Task)) (domain.Task, error) {
	if git == nil {
		git = gitops.DefaultGit
	}
	t, err := s.LoadTask(taskID)
	if err != nil {
		return t, err
	}
	if t.State == domain.TaskRunning {
		return t, taskError("task is running; cancel it first")
	}
	if t.State == domain.TaskInReview && onRejected != nil {
		onRejected(t)
	}
	if t.Worktree != nil {
		if err := gitops.RemoveWorktree(s.Root, t.ID, git); err != nil {
			return t, err
		}
	}
	t.State, t.Worktree, t.Branch = domain.TaskDiscarded, nil, nil
	if t, err = s.SaveTask(t, nil); err != nil {
		return t, err
	}
	return t, s.Audit(store.AuditEvent{Type: "task.discarded", TaskID: taskID})
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// roundSeconds rounds a duration to tenths of a second for the run record.
func roundSeconds(d time.Duration) float64 { return math.Round(d.Seconds()*10) / 10 }

// RunSummaryLimit caps the agent's final text kept in the task record. A research answer must fit;
// the run log keeps the full stream either way.
const RunSummaryLimit = 50000
