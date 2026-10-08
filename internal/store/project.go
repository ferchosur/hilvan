package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/security"
)

// StateDir is the project's state folder.
const StateDir = ".hilvan"

const maxJSON = 4 * 1024 * 1024

// Project is .hilvan/project.json.
// Field order matches the TypeScript output ({ version, createdAt, ...project }).
type Project struct {
	Version    int    `json:"version"`
	CreatedAt  string `json:"createdAt"`
	Name       string `json:"name"`
	BaseBranch string `json:"baseBranch"`
	// SetupHook is an optional command run in each new worktree (e.g. npm ci), without a shell.
	SetupHook []string `json:"setupHook,omitempty"`
}

// ProjectError is a user-facing problem with the project state.
type ProjectError struct{ Msg string }

func (e *ProjectError) Error() string { return e.Msg }

func projectError(format string, args ...any) error {
	return &ProjectError{Msg: fmt.Sprintf(format, args...)}
}

var (
	taskID    = regexp.MustCompile(`^tsk_[0-9A-Z]{26}$`)
	runID     = regexp.MustCompile(`^run_[0-9A-Z]{26}$`)
	handoffID = regexp.MustCompile(`^ho_[0-9A-Z]{26}$`)
)

// Now returns the time as JavaScript's toISOString does (millisecond precision, UTC).
func Now() string { return time.Now().UTC().Format("2006-01-02T15:04:05.000Z") }

// MarshalIndent is JSON.stringify(v, null, 2) + "\n": two-space indent, no HTML escaping.
func MarshalIndent(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// ProjectStore is the repository over a project's .hilvan/ folder.
type ProjectStore struct {
	Root string
	Dir  string
}

// NewProjectStore returns the store of a project root.
func NewProjectStore(root string) *ProjectStore {
	return &ProjectStore{Root: root, Dir: filepath.Join(root, StateDir)}
}

// Find walks up from cwd to the nearest folder with .hilvan/project.json.
func Find(cwd string) (*ProjectStore, error) {
	dir, err := filepath.Abs(cwd)
	if err != nil {
		return nil, err
	}
	for {
		if _, found, err := security.ReadTextSafe(filepath.Join(dir, StateDir, "project.json"), maxJSON); err == nil && found {
			return NewProjectStore(dir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, projectError("not a Hilvan project (run `hilvan init`)")
		}
		dir = parent
	}
}

// Init creates .hilvan/ with project.json and a .gitignore that keeps it out of git (SR-016).
func (s *ProjectStore) Init(name, baseBranch string, setupHook []string) (Project, error) {
	p := Project{Version: 1, Name: name, BaseBranch: baseBranch, SetupHook: setupHook, CreatedAt: Now()}
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return p, err
	}
	if err := security.WriteFileAtomic(filepath.Join(s.Dir, ".gitignore"), []byte("*\n"), 0o644); err != nil {
		return p, err
	}
	data, err := MarshalIndent(p)
	if err != nil {
		return p, err
	}
	return p, security.WriteFileAtomic(filepath.Join(s.Dir, "project.json"), data, 0o600)
}

// Project reads project.json.
func (s *ProjectStore) Project() (Project, error) {
	var p Project
	text, found, err := security.ReadTextSafe(filepath.Join(s.Dir, "project.json"), maxJSON)
	if err != nil {
		return p, err
	}
	if !found {
		return p, projectError("missing .hilvan/project.json")
	}
	if err := json.Unmarshal([]byte(text), &p); err != nil {
		return p, projectError("invalid .hilvan/project.json: %v", err)
	}
	if p.Version != 1 || p.Name == "" || p.BaseBranch == "" || !domain.IsISODateTime(p.CreatedAt) {
		return p, projectError("invalid .hilvan/project.json")
	}
	return p, nil
}

// TaskDir is a task's folder; the id is validated so it can never escape .hilvan/.
func (s *ProjectStore) TaskDir(id string) (string, error) {
	if !taskID.MatchString(id) {
		return "", projectError("invalid task id %q", id)
	}
	return filepath.Join(s.Dir, "tasks", id), nil
}

func (s *ProjectStore) taskFile(taskIDv, sub, id string, re *regexp.Regexp, kind, ext string) (string, error) {
	dir, err := s.TaskDir(taskIDv)
	if err != nil {
		return "", err
	}
	if !re.MatchString(id) {
		return "", projectError("invalid %s id %q", kind, id)
	}
	return filepath.Join(dir, sub, id+ext), nil
}

// RunFile is the redacted JSONL log of a run.
func (s *ProjectStore) RunFile(taskID, runIDv string) (string, error) {
	return s.taskFile(taskID, "runs", runIDv, runID, "run", ".jsonl")
}

// HandoffFile is an approved hand-off package.
func (s *ProjectStore) HandoffFile(taskID, id string) (string, error) {
	return s.taskFile(taskID, "handoffs", id, handoffID, "hand-off", ".md")
}

// ReviewFile is a cross review's text.
func (s *ProjectStore) ReviewFile(taskID, runIDv string) (string, error) {
	return s.taskFile(taskID, "reviews", runIDv, runID, "run", ".md")
}

// SaveTask validates and writes task.json (updating updatedAt) and, if given, brief.md.
func (s *ProjectStore) SaveTask(t domain.Task, brief *string) (domain.Task, error) {
	t.UpdatedAt = Now()
	t.Normalize()
	if issues := t.Validate(); len(issues) > 0 {
		return t, fmt.Errorf("invalid task: %s", issues.Pretty())
	}
	dir, err := s.TaskDir(t.ID)
	if err != nil {
		return t, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "runs"), 0o700); err != nil {
		return t, err
	}
	data, err := MarshalIndent(t)
	if err != nil {
		return t, err
	}
	if err := security.WriteFileAtomic(filepath.Join(dir, "task.json"), data, 0o600); err != nil {
		return t, err
	}
	if brief != nil {
		if err := security.WriteFileAtomic(filepath.Join(dir, "brief.md"), []byte(*brief), 0o600); err != nil {
			return t, err
		}
	}
	return t, nil
}

// LoadTask reads and validates task.json.
func (s *ProjectStore) LoadTask(id string) (domain.Task, error) {
	var t domain.Task
	dir, err := s.TaskDir(id)
	if err != nil {
		return t, err
	}
	text, found, err := security.ReadTextSafe(filepath.Join(dir, "task.json"), maxJSON)
	if err != nil {
		return t, err
	}
	if !found {
		return t, projectError("no task %q", id)
	}
	if err := json.Unmarshal([]byte(text), &t); err != nil {
		return t, projectError("invalid task %s: %v", id, err)
	}
	t.Normalize()
	if issues := t.Validate(); len(issues) > 0 {
		return t, projectError("invalid task %s: %s", id, issues.Pretty())
	}
	return t, nil
}

func (s *ProjectStore) readOptional(file string) (string, error) {
	text, _, err := security.ReadTextSafe(file, maxJSON)
	return text, err
}

// Brief returns brief.md ("" when missing).
func (s *ProjectStore) Brief(id string) (string, error) {
	dir, err := s.TaskDir(id)
	if err != nil {
		return "", err
	}
	return s.readOptional(filepath.Join(dir, "brief.md"))
}

// Handoff returns an approved hand-off package.
func (s *ProjectStore) Handoff(taskID, id string) (string, error) {
	file, err := s.HandoffFile(taskID, id)
	if err != nil {
		return "", err
	}
	text, found, err := security.ReadTextSafe(file, maxJSON)
	if err != nil {
		return "", err
	}
	if !found {
		return "", projectError("missing hand-off %s", id)
	}
	return text, nil
}

// TaskIDs lists task ids in creation order.
func (s *ProjectStore) TaskIDs() []string {
	entries, err := os.ReadDir(filepath.Join(s.Dir, "tasks"))
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range entries {
		if taskID.MatchString(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	sort.Strings(ids)
	return ids
}

// ResolveTaskID accepts a full id or a unique suffix/prefix (e.g. the last 6 characters shown).
func (s *ProjectStore) ResolveTaskID(ref string) (string, error) {
	if taskID.MatchString(ref) {
		return ref, nil
	}
	var hits []string
	for _, id := range s.TaskIDs() {
		if strings.HasSuffix(id, strings.ToUpper(ref)) || strings.HasPrefix(id, ref) {
			hits = append(hits, id)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return "", projectError("no task %q", ref)
	}
	return "", projectError("ambiguous task %q", ref)
}

// ListTasks loads every task.
func (s *ProjectStore) ListTasks() ([]domain.Task, error) {
	var out []domain.Task
	for _, id := range s.TaskIDs() {
		t, err := s.LoadTask(id)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// Audit appends an event with the current time to .hilvan/events.jsonl.
func (s *ProjectStore) Audit(ev AuditEvent) error {
	ev.TS = Now()
	_, err := AppendAudit(filepath.Join(s.Dir, "events.jsonl"), ev)
	return err
}

// Lock is the single active run of a project (MVP: one run at a time).
type Lock struct {
	PID       int    `json:"pid"`
	TaskID    string `json:"taskId"`
	RunID     string `json:"runId"`
	StartedAt string `json:"startedAt"`
}

// LockFile is .hilvan/lock.
func (s *ProjectStore) LockFile() string { return filepath.Join(s.Dir, "lock") }

// AcquireLock creates the lock exclusively or reports who holds it.
func (s *ProjectStore) AcquireLock(taskID, runID string) error {
	body, _ := json.Marshal(Lock{PID: os.Getpid(), TaskID: taskID, RunID: runID, StartedAt: Now()})
	f, err := os.OpenFile(s.LockFile(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		held := s.ReadLock()
		if held == nil {
			return projectError("another run is active (unknown for ?, pid ?)")
		}
		return projectError("another run is active (%s for %s, pid %d)", held.RunID, held.TaskID, held.PID)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(body)
	return err
}

// ReadLock returns the current lock, or nil.
func (s *ProjectStore) ReadLock() *Lock {
	text, found, err := security.ReadTextSafe(s.LockFile(), 4096)
	if err != nil || !found {
		return nil
	}
	var l Lock
	if json.Unmarshal([]byte(text), &l) != nil {
		return nil
	}
	return &l
}

// ReleaseLock removes the lock.
func (s *ProjectStore) ReleaseLock() error {
	err := os.Remove(s.LockFile())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
