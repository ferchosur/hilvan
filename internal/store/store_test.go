package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/domain"
)

// The chain in testdata/events.ts.jsonl was written by the TypeScript implementation
// (scripts/parity/gen-g2.ts): Go must verify it and compute the same hashes.
func TestVerifiesTypeScriptChain(t *testing.T) {
	r, err := VerifyAudit("testdata/events.ts.jsonl", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !r.OK || r.Events != 5 {
		t.Fatalf("TypeScript chain: %+v", r)
	}
	raw, _ := os.ReadFile("testdata/events.ts.jsonl")
	prev := Genesis
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var ev map[string]any
		json.Unmarshal([]byte(line), &ev)
		want := ev["hash"].(string)
		delete(ev, "hash")
		delete(ev, "prev")
		if got := ChainHash(prev, ev); got != want {
			t.Fatalf("hash %s, want %s for %s", got, want, line)
		}
		prev = want
	}
}

// Go continues a chain started by TypeScript.
func TestAppendsToTypeScriptChain(t *testing.T) {
	file := filepath.Join(t.TempDir(), "events.jsonl")
	raw, _ := os.ReadFile("testdata/events.ts.jsonl")
	os.WriteFile(file, raw, 0o600)
	if _, err := AppendAudit(file, AuditEvent{TS: "2026-10-07T00:00:05.000Z", Type: "go.event", Data: map[string]any{"n": 1.5, "s": "<&>"}}); err != nil {
		t.Fatal(err)
	}
	if r, _ := VerifyAudit(file, 0); !r.OK || r.Events != 6 {
		t.Fatalf("mixed chain: %+v", r)
	}
}

func ev(i int) AuditEvent {
	return AuditEvent{TS: "2026-10-07T00:00:0" + string(rune('0'+i)) + "Z", Type: "run.started", TaskID: "t" + string(rune('0'+i)),
		Data: map[string]any{"sha256": strings.Repeat("ab", 32), "n": i}}
}

func TestDetectsTampering(t *testing.T) {
	file := filepath.Join(t.TempDir(), ".hilvan", "events.jsonl")
	for i := 0; i < 4; i++ {
		if _, err := AppendAudit(file, ev(i)); err != nil {
			t.Fatal(err)
		}
	}
	if r, _ := VerifyAudit(file, 0); !r.OK || r.Events != 4 {
		t.Fatalf("intact chain: %+v", r)
	}
	raw, _ := os.ReadFile(file)
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	write := func(ls ...string) { os.WriteFile(file, []byte(strings.Join(ls, "\n")+"\n"), 0o600) }

	write(append([]string{lines[0], strings.Replace(lines[1], `"t1"`, `"tX"`, 1)}, lines[2:]...)...)
	if r, _ := VerifyAudit(file, 0); r.OK || r.Broken != 2 || r.Reason != "content was modified" {
		t.Errorf("edit: %+v", r)
	}
	write(append([]string{lines[0]}, lines[2:]...)...)
	if r, _ := VerifyAudit(file, 0); r.OK || r.Broken != 2 || r.Reason != "chain link mismatch" {
		t.Errorf("deletion: %+v", r)
	}
	write(append([]string{lines[1], lines[0]}, lines[2:]...)...)
	if r, _ := VerifyAudit(file, 0); r.OK {
		t.Error("reordering not detected")
	}
	write(lines...)
	if r, _ := VerifyAudit(file, 0); !r.OK {
		t.Error("restored chain")
	}
	if r, _ := VerifyAudit(filepath.Join(t.TempDir(), "none.jsonl"), 0); !r.OK || r.Events != 0 {
		t.Errorf("missing log: %+v", r)
	}
}

func TestProjectStore(t *testing.T) {
	root := t.TempDir()
	s := NewProjectStore(root)
	if _, err := s.Init("repo", "main", nil); err != nil {
		t.Fatal(err)
	}
	if gi, _ := os.ReadFile(filepath.Join(root, ".hilvan", ".gitignore")); string(gi) != "*\n" {
		t.Errorf(".gitignore %q", gi)
	}
	raw, _ := os.ReadFile(filepath.Join(root, ".hilvan", "project.json"))
	if !strings.HasPrefix(string(raw), "{\n  \"version\": 1,\n  \"createdAt\": ") {
		t.Errorf("project.json shape:\n%s", raw)
	}
	sub := filepath.Join(root, "a", "b")
	os.MkdirAll(sub, 0o700)
	found, err := Find(sub)
	if err != nil || found.Root != root {
		t.Fatalf("find from a subfolder: %v %v", found, err)
	}
	if _, err := Find(t.TempDir()); err == nil {
		t.Error("found a project where there is none")
	}

	at := Now()
	task := domain.Task{ID: "tsk_01JABCDEFGHJKMNPQRSTVWXYZ0", Title: "T", Classification: "personal", Mode: "read", State: domain.TaskDraft, CreatedAt: at, UpdatedAt: at}
	brief := "# Brief\n"
	if _, err := s.SaveTask(task, &brief); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadTask(task.ID)
	if err != nil || loaded.Title != "T" || loaded.Requires == nil {
		t.Fatalf("load: %+v %v", loaded, err)
	}
	if b, _ := s.Brief(task.ID); b != brief {
		t.Errorf("brief %q", b)
	}
	if id, err := s.ResolveTaskID("WXYZ0"); err != nil || id != task.ID {
		t.Errorf("resolve suffix: %q %v", id, err)
	}
	if _, err := s.ResolveTaskID("nope"); err == nil {
		t.Error("resolved a missing task")
	}
	if _, err := s.TaskDir("../../etc"); err == nil {
		t.Error("path traversal through a task id")
	}
	bad := task
	bad.Mode = "delete"
	if _, err := s.SaveTask(bad, nil); err == nil {
		t.Error("invalid task saved")
	}

	if err := s.AcquireLock(task.ID, "run_01JABCDEFGHJKMNPQRSTVWXYZ1"); err != nil {
		t.Fatal(err)
	}
	if err := s.AcquireLock(task.ID, "run_01JABCDEFGHJKMNPQRSTVWXYZ2"); err == nil || !strings.Contains(err.Error(), "another run is active (run_01JABCDEFGHJKMNPQRSTVWXYZ1") {
		t.Errorf("second lock: %v", err)
	}
	if l := s.ReadLock(); l == nil || l.PID != os.Getpid() || !ProcessAlive(l.PID) {
		t.Errorf("lock %+v", l)
	}
	s.ReleaseLock()
	if s.ReadLock() != nil || s.ReleaseLock() != nil {
		t.Error("release")
	}

	if err := s.Audit(AuditEvent{Type: "task.created", TaskID: task.ID}); err != nil {
		t.Fatal(err)
	}
	if r, _ := VerifyAudit(filepath.Join(root, ".hilvan", "events.jsonl"), 0); !r.OK || r.Events != 1 {
		t.Errorf("audit: %+v", r)
	}
}
