package cli

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
)

// Without --profile, task run asks Criterio. With no eligible profile it stops before running and
// says how to see why (AC-013: no silent choice).
func TestTaskRunWithoutProfileNeedsEligibleProfile(t *testing.T) {
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
	if _, errOut, code := run(t, app, "task", "new", "Lectura", "--objective", "o", "--criteria", "c", "--mode", "read"); code != 0 {
		t.Fatalf("task new: %s", errOut)
	}
	list, _, _ := run(t, app, "task", "list")
	id := strings.Fields(list)[0]
	_, errOut, code := run(t, app, "task", "run", id)
	if code == 0 || !strings.Contains(errOut, "no profile is eligible") || !strings.Contains(errOut, "task recommend") {
		t.Errorf("task run without profiles: code %d, %q", code, errOut)
	}
}
