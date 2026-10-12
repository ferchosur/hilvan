package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AcquireProfileLock takes the lock of one profile across projects (D-027): a profile runs one
// task at a time, wherever the task belongs. A lock held by a live process is busy; a lock left
// by a process that no longer exists is stale and is replaced.
func AcquireProfileLock(dir, profileID string, lock Lock) error {
	path, err := profileLockPath(dir, profileID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	body, _ := json.Marshal(lock)
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_, werr := f.Write(body)
			f.Close()
			return werr
		}
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
		held := readProfileLock(path)
		if held != nil && ProcessAlive(held.PID) {
			return fmt.Errorf("profile %s is busy: task %s is running there (run %s, pid %d); wait for it to finish or cancel it",
				profileID, held.TaskID, held.RunID, held.PID)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return fmt.Errorf("profile %s is busy: could not take its lock", profileID)
}

// ReleaseProfileLock removes the profile lock if it still belongs to this run.
func ReleaseProfileLock(dir, profileID, runID string) error {
	path, err := profileLockPath(dir, profileID)
	if err != nil {
		return err
	}
	if held := readProfileLock(path); held == nil || held.RunID != runID {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func profileLockPath(dir, profileID string) (string, error) {
	if profileID == "" || strings.ContainsAny(profileID, `/\`) || profileID == "." || profileID == ".." {
		return "", fmt.Errorf("invalid profile id %q for a lock", profileID)
	}
	return filepath.Join(dir, profileID+".lock"), nil
}

func readProfileLock(path string) *Lock {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var l Lock
	if json.Unmarshal(data, &l) != nil {
		return nil
	}
	return &l
}
