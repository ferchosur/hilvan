package gitops

import (
	"reflect"
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/process"
)

func fake(outputs map[string]string) Git {
	return func(args []string, _ string) (process.Result, error) {
		out, ok := outputs[strings.Join(args, " ")]
		if !ok {
			return process.Result{Code: 1, Stderr: "unexpected: " + strings.Join(args, " ")}, nil
		}
		return process.Result{Stdout: out}, nil
	}
}

func TestNamesComeFromValidatedIDs(t *testing.T) {
	if b, err := TaskBranch("tsk_01JABCDEFGHJKMNPQRSTVWXYZ0"); err != nil || b != "hilvan/tsk_01JABCDEFGHJKMNPQRSTVWXYZ0" {
		t.Errorf("branch %q %v", b, err)
	}
	for _, bad := range []string{"../x", "tsk_1", "main"} {
		if _, err := WorktreePath("/r", bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestDiffSummaryHandlesRenamesAndBinaries(t *testing.T) {
	out := "3\t1\tsrc/a.ts\x00-\t-\timg.png\x002\t0\t\x00old/b.ts\x00new/b.ts\x00"
	d, err := DiffSummary("/r", "main", "hilvan/x", fake(map[string]string{"diff --numstat -z main...hilvan/x": out}))
	if err != nil {
		t.Fatal(err)
	}
	if d.FilesChanged != 3 || d.LinesAdded != 5 || d.LinesDeleted != 1 || !reflect.DeepEqual(d.Paths, []string{"src/a.ts", "img.png", "new/b.ts"}) {
		t.Errorf("summary %+v", d)
	}
}

func TestRelativePathsNeedGit248(t *testing.T) {
	for version, want := range map[string]bool{
		"git version 2.53.0":           true,
		"git version 2.48.1":           true,
		"git version 2.55.0.windows.3": true,
		"git version 2.47.2":           false,
		"git version 3.0.0":            true,
		"something else":               false,
	} {
		if got := supportsRelativePaths(fake(map[string]string{"--version": version}), "/r"); got != want {
			t.Errorf("%s: %v", version, got)
		}
	}
}

func TestFallbackIdentity(t *testing.T) {
	none := fake(map[string]string{"config user.name": "", "config user.email": ""})
	if got := FallbackIdentity("/r", none); len(got) != 4 {
		t.Errorf("no identity: %v", got)
	}
	nameOnly := fake(map[string]string{"config user.name": "Ana", "config user.email": ""})
	if got := FallbackIdentity("/r", nameOnly); len(got) != 4 {
		t.Errorf("missing email: %v", got)
	}
	full := fake(map[string]string{"config user.name": "Ana", "config user.email": "a@b"})
	if got := FallbackIdentity("/r", full); got != nil {
		t.Errorf("user identity overridden: %v", got)
	}
}
