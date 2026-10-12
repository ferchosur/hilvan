package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// D-027: a live holder makes the profile busy; a stale lock (dead pid) is replaced.
func TestProfileLockBusyAndStale(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "locks")
	mine := Lock{PID: os.Getpid(), TaskID: "tsk_a", RunID: "run_a", StartedAt: Now()}
	if err := AcquireProfileLock(dir, "claude-a", mine); err != nil {
		t.Fatal(err)
	}
	err := AcquireProfileLock(dir, "claude-a", Lock{PID: os.Getpid(), TaskID: "tsk_b", RunID: "run_b"})
	if err == nil || !strings.Contains(err.Error(), "busy") || !strings.Contains(err.Error(), "tsk_a") {
		t.Fatalf("busy profile accepted: %v", err)
	}
	if err := ReleaseProfileLock(dir, "claude-a", "run_b"); err != nil {
		t.Fatal(err)
	}
	if held := readProfileLock(filepath.Join(dir, "claude-a.lock")); held == nil || held.RunID != "run_a" {
		t.Fatal("a run released a lock it does not own")
	}
	if err := os.WriteFile(filepath.Join(dir, "claude-b.lock"), []byte(`{"pid":2147483646,"taskId":"tsk_old","runId":"run_old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AcquireProfileLock(dir, "claude-b", Lock{PID: os.Getpid(), TaskID: "tsk_new", RunID: "run_new"}); err != nil {
		t.Fatalf("stale lock not replaced: %v", err)
	}
	if _, err := profileLockPath(dir, "../x"); err == nil {
		t.Error("profile id with a path separator accepted")
	}
}
