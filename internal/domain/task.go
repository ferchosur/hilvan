package domain

import (
	"regexp"
	"strconv"
)

// TaskTypes are the kinds of work Criterio recognises (ADR-0011), in model class order.
var TaskTypes = []string{"implement", "fix", "refactor", "test", "review", "research", "docs"}

// TaskState is the task lifecycle (docs/design/modelo-de-datos.md §5).
type TaskState = string

const (
	TaskDraft            TaskState = "draft"
	TaskReady            TaskState = "ready"
	TaskAwaitingApproval TaskState = "awaiting_approval"
	TaskRunning          TaskState = "running"
	TaskInReview         TaskState = "in_review"
	TaskFailed           TaskState = "failed"
	TaskBlocked          TaskState = "blocked"
	TaskUncertain        TaskState = "uncertain"
	TaskAccepted         TaskState = "accepted"
	TaskDiscarded        TaskState = "discarded"
)

var transitions = map[TaskState][]TaskState{
	TaskDraft:            {TaskReady},
	TaskReady:            {TaskAwaitingApproval, TaskRunning},
	TaskAwaitingApproval: {TaskReady, TaskDraft},
	TaskRunning:          {TaskInReview, TaskFailed, TaskBlocked, TaskUncertain, TaskReady},
	TaskInReview:         {TaskReady, TaskAccepted, TaskDiscarded},
	TaskFailed:           {TaskReady},
	TaskBlocked:          {TaskReady},
	TaskUncertain:        {TaskReady},
	TaskAccepted:         {},
	TaskDiscarded:        {},
}

// CanTransition reports whether a task may move from one state to another.
func CanTransition(from, to TaskState) bool { return contains(transitions[from], to) }

// IsTerminal reports whether a state has no way out (accepted, discarded).
func IsTerminal(s TaskState) bool {
	next, ok := transitions[s]
	return ok && len(next) == 0
}

// Run states and blocked reasons.
const (
	RunRunning   = "running"
	RunCompleted = "completed"
	RunFailed    = "failed"
	RunBlocked   = "blocked"
	RunCancelled = "cancelled"
	RunUncertain = "uncertain"
)

// Usage is what the agent CLI reported; always a client-side estimate (FR-016).
type Usage struct {
	CostUSD      *float64 `json:"costUsd,omitempty"`
	InputTokens  *int64   `json:"inputTokens,omitempty"`
	OutputTokens *int64   `json:"outputTokens,omitempty"`
	Estimated    bool     `json:"estimated"`
}

// LimitWindow is one quota window reported by the CLI (e.g. five_hour).
type LimitWindow struct {
	Name        string   `json:"name"`
	Utilization float64  `json:"utilization"`
	ResetsAt    *float64 `json:"resetsAt,omitempty"`
}

// DiffSummary are the facts of a run's changes.
type DiffSummary struct {
	FilesChanged int      `json:"filesChanged"`
	LinesAdded   int      `json:"linesAdded"`
	LinesDeleted int      `json:"linesDeleted"`
	Paths        []string `json:"paths"`
}

// RunSummary is one execution of a task by a profile.
type RunSummary struct {
	ID            string        `json:"id"`
	ProfileID     string        `json:"profileId"`
	State         string        `json:"state"`
	BlockedReason string        `json:"blockedReason,omitempty"`
	SessionID     string        `json:"sessionId,omitempty"`
	Usage         *Usage        `json:"usage,omitempty"`
	Limits        []LimitWindow `json:"limits,omitempty"`
	Diff          *DiffSummary  `json:"diff,omitempty"`
	// Risk is Criterio's diff risk, 2..10.
	Risk *float64 `json:"risk,omitempty"`
	// MeetsCriteria is Criterio's estimate that the run met the brief.
	MeetsCriteria *float64 `json:"meetsCriteria,omitempty"`
	Error         string   `json:"error,omitempty"`
	// Summary is the agent's final summary, redacted and truncated. An unverified claim.
	Summary   string `json:"summary,omitempty"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt,omitempty"`
}

// Review is a cross review attached to a task (FR-015).
type Review struct {
	ID        string `json:"id"`
	ProfileID string `json:"profileId"`
	Verdict   string `json:"verdict"`
	Usage     *Usage `json:"usage,omitempty"`
	At        string `json:"at"`
}

// Task is the content of .hilvan/tasks/<id>/task.json. Field order matches the TypeScript output.
type Task struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Classification string `json:"classification"`
	// Mode is write (own git worktree, ADR-0008) or read.
	Mode          string       `json:"mode"`
	Requires      []string     `json:"requires"`
	TaskType      string       `json:"taskType,omitempty"`
	Complexity    *float64     `json:"complexity,omitempty"`
	CriteriaCount int          `json:"criteriaCount"`
	ProfileID     *string      `json:"profileId"`
	BaseBranch    *string      `json:"baseBranch"`
	State         TaskState    `json:"state"`
	Branch        *string      `json:"branch"`
	Worktree      *string      `json:"worktree"`
	Runs          []RunSummary `json:"runs"`
	Handoffs      []string     `json:"handoffs"`
	Reviews       []Review     `json:"reviews"`
	// ReviewedHead is the branch head whose diff the user reviewed (SR-024).
	ReviewedHead *string `json:"reviewedHead"`
	// ApprovedProfiles already received the user's approval for this task's brief.
	ApprovedProfiles []string `json:"approvedProfiles"`
	CreatedAt        string   `json:"createdAt"`
	UpdatedAt        string   `json:"updatedAt"`
}

// Normalize fills defaults so the JSON output has the same shape as the TypeScript one.
func (t *Task) Normalize() {
	if t.Requires == nil {
		t.Requires = []string{}
	}
	if t.Runs == nil {
		t.Runs = []RunSummary{}
	}
	if t.Handoffs == nil {
		t.Handoffs = []string{}
	}
	if t.Reviews == nil {
		t.Reviews = []Review{}
	}
	if t.ApprovedProfiles == nil {
		t.ApprovedProfiles = []string{}
	}
	for i := range t.Runs {
		if d := t.Runs[i].Diff; d != nil && d.Paths == nil {
			d.Paths = []string{}
		}
	}
}

var (
	taskIDPattern    = regexp.MustCompile(`^tsk_[0-9A-Z]{26}$`)
	runIDPattern     = regexp.MustCompile(`^run_[0-9A-Z]{26}$`)
	handoffIDPattern = regexp.MustCompile(`^ho_[0-9A-Z]{26}$`)
	isoDateTime      = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})T([01]\d|2[0-3]):[0-5]\d:[0-5]\d(\.\d+)?Z$`)
)

// IsISODateTime mirrors zod's z.iso.datetime(): UTC "Z" only, seconds required, real calendar days.
func IsISODateTime(s string) bool {
	m := isoDateTime.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	if mo < 1 || mo > 12 || d < 1 {
		return false
	}
	days := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[mo-1]
	if mo == 2 && (y%4 == 0 && (y%100 != 0 || y%400 == 0)) {
		days = 29
	}
	return d <= days
}

func (b *issues) datetime(path, v string) {
	if !IsISODateTime(v) {
		b.add(path, "Invalid ISO datetime")
	}
}

func (b *issues) usage(path string, u *Usage) {
	if u == nil {
		return
	}
	if u.CostUSD != nil && *u.CostUSD < 0 {
		b.add(field(path, "costUsd"), "Too small: expected number to be >=0")
	}
	if u.InputTokens != nil && *u.InputTokens < 0 {
		b.add(field(path, "inputTokens"), "Too small: expected int to be >=0")
	}
	if u.OutputTokens != nil && *u.OutputTokens < 0 {
		b.add(field(path, "outputTokens"), "Too small: expected int to be >=0")
	}
	if !u.Estimated {
		b.add(field(path, "estimated"), "Invalid input: expected true")
	}
}

func (b *issues) between(path string, v *float64, lo, hi float64) {
	if v != nil && (*v < lo || *v > hi) {
		b.add(path, "must be between %s and %s", strconv.FormatFloat(lo, 'f', -1, 64), strconv.FormatFloat(hi, 'f', -1, 64))
	}
}

func (r *RunSummary) validate(b *issues, path string) {
	b.pattern(field(path, "id"), r.ID, runIDPattern)
	b.id(field(path, "profileId"), r.ProfileID)
	b.oneOf(field(path, "state"), r.State, RunRunning, RunCompleted, RunFailed, RunBlocked, RunCancelled, RunUncertain)
	if r.BlockedReason != "" {
		b.oneOf(field(path, "blockedReason"), r.BlockedReason, "rate_limit", "auth", "billing", "permission")
	}
	b.usage(field(path, "usage"), r.Usage)
	for i, l := range r.Limits {
		u := l.Utilization
		b.between(field(index(field(path, "limits"), i), "utilization"), &u, 0, 1)
	}
	if d := r.Diff; d != nil {
		dp := field(path, "diff")
		if d.FilesChanged < 0 || d.LinesAdded < 0 || d.LinesDeleted < 0 {
			b.add(dp, "diff counts must be >=0")
		}
		if len(d.Paths) > 2000 {
			b.add(field(dp, "paths"), "Too big: expected array to have <=2000 items")
		}
	}
	b.between(field(path, "risk"), r.Risk, 2, 10)
	b.between(field(path, "meetsCriteria"), r.MeetsCriteria, 0, 1)
	if utf16Len(r.Error) > 2000 {
		b.add(field(path, "error"), "Too big: expected string to have <=2000 characters")
	}
	if utf16Len(r.Summary) > 4000 {
		b.add(field(path, "summary"), "Too big: expected string to have <=4000 characters")
	}
	b.datetime(field(path, "startedAt"), r.StartedAt)
	if r.EndedAt != "" {
		b.datetime(field(path, "endedAt"), r.EndedAt)
	}
}

// Validate checks a normalized task.
func (t *Task) Validate() Issues {
	var b issues
	b.pattern("id", t.ID, taskIDPattern)
	b.nonEmpty("title", t.Title)
	b.nonEmpty("classification", t.Classification)
	b.oneOf("mode", t.Mode, "write", "read")
	b.ids("requires", t.Requires)
	if t.TaskType != "" {
		b.oneOf("taskType", t.TaskType, TaskTypes...)
	}
	b.between("complexity", t.Complexity, 2, 10)
	if t.CriteriaCount < 0 {
		b.add("criteriaCount", "Too small: expected int to be >=0")
	}
	if t.ProfileID != nil {
		b.id("profileId", *t.ProfileID)
	}
	if _, ok := transitions[t.State]; !ok {
		b.add("state", "Invalid task state %q", t.State)
	}
	for i := range t.Runs {
		t.Runs[i].validate(&b, index("runs", i))
	}
	for i, h := range t.Handoffs {
		b.pattern(index("handoffs", i), h, handoffIDPattern)
	}
	for i, r := range t.Reviews {
		path := index("reviews", i)
		b.pattern(field(path, "id"), r.ID, runIDPattern)
		b.id(field(path, "profileId"), r.ProfileID)
		b.oneOf(field(path, "verdict"), r.Verdict, "approve", "changes_requested", "unknown")
		b.usage(field(path, "usage"), r.Usage)
		b.datetime(field(path, "at"), r.At)
	}
	b.ids("approvedProfiles", t.ApprovedProfiles)
	b.datetime("createdAt", t.CreatedAt)
	b.datetime("updatedAt", t.UpdatedAt)
	return b.list
}
