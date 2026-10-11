package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/store"
)

func TestRenderWatchEmptyAndSanitized(t *testing.T) {
	now := time.Date(2026, 10, 11, 9, 30, 5, 0, time.UTC)
	if out := renderWatch("demo", nil, nil, now); !strings.Contains(out, "No tasks yet") || !strings.Contains(out, "09:30:05") {
		t.Errorf("empty frame: %q", out)
	}
	profile := "claude-personal"
	tasks := []domain.Task{{ID: "tsk_01JABCDEFGHJKMNPQRSTVWXY01", Title: "Arregla\x1b[31m el log\x1b[0m", State: "running", ProfileID: &profile}}
	lock := &store.Lock{TaskID: "tsk_01JABCDEFGHJKMNPQRSTVWXY01", RunID: "run_01JABCDEFGHJKMNPQRSTVWXY02", StartedAt: "2026-10-11T09:29:00Z"}
	out := renderWatch("demo", tasks, lock, now)
	if strings.Contains(out, "\x1b") {
		t.Errorf("escape sequences must be stripped: %q", out)
	}
	for _, want := range []string{"Running: task", "running", "claude-personal", "Arregla el log"} {
		if !strings.Contains(out, want) {
			t.Errorf("frame lacks %q:\n%s", want, out)
		}
	}
}

// watch --once must print the tasks and leave .hilvan/ byte-for-byte unchanged.
func TestWatchIsReadOnly(t *testing.T) {
	tmp := t.TempDir()
	repo := filepath.Join(tmp, "repo")
	if out, err := exec.Command("git", "init", "-q", "-b", "main", repo).CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "i").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	host := core.CurrentHost()
	host.InsideWSL = false
	platform := config.Platform{OS: host.OS, Env: map[string]string{"HILVAN_CONFIG_DIR": filepath.Join(tmp, "config")}, Homedir: tmp}
	app := &App{Stdin: strings.NewReader(""), Cwd: repo, Platform: platform, Host: host}

	if _, _, code := run(t, app, "init"); code != 0 {
		t.Fatal("init failed")
	}
	if _, errOut, code := run(t, app, "task", "new", "Arregla \x1b[31mel log", "--objective", "o", "--criteria", "c"); code != 0 {
		t.Fatalf("task new: %s", errOut)
	}
	before := snapshot(t, filepath.Join(repo, ".hilvan"))

	out, errOut, code := run(t, app, "watch", "--once")
	if code != 0 || errOut != "" {
		t.Fatalf("watch --once: code %d, stderr %q", code, errOut)
	}
	must(t, `Hilvan · repo · \d\d:\d\d:\d\d`, out)
	must(t, `ready +5QTCZP|ready +[A-Z0-9]{6}`, out)
	if strings.Contains(out, "\x1b") {
		t.Errorf("title escape sequences leaked into the frame: %q", out)
	}
	if after := snapshot(t, filepath.Join(repo, ".hilvan")); after != before {
		t.Errorf(".hilvan changed during watch:\nbefore %s\nafter  %s", before, after)
	}

	if _, errOut, code := run(t, app, "watch", "--interval", "0"); code == 0 || !strings.Contains(errOut, "--interval") {
		t.Errorf("interval 0 must be rejected: code %d, %q", code, errOut)
	}
}

// snapshot concatenates every file under dir with its path, to detect any write.
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		b.WriteString(p + "\x00" + string(data) + "\x00")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
