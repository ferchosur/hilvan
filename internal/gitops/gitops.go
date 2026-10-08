// Package gitops isolates each write task in its own worktree and branch (ADR-0008, SR-003,
// SR-023). Branch and worktree names come only from validated task ids.
package gitops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/process"
)

// GitError is a failed git operation.
type GitError struct{ Msg string }

func (e *GitError) Error() string { return e.Msg }

// IsGitError reports whether err is a GitError.
func IsGitError(err error) bool {
	var g *GitError
	return errors.As(err, &g)
}

// Git runs git with args in cwd.
type Git func(args []string, cwd string) (process.Result, error)

// DefaultGit never prompts for credentials or opens editors from a background run.
func DefaultGit(args []string, cwd string) (process.Result, error) {
	env := process.CurrentEnv()
	env["GIT_TERMINAL_PROMPT"] = "0"
	env["GIT_EDITOR"] = "true"
	return process.Run("git", args, process.Options{Dir: cwd, Timeout: 60 * time.Second, Env: env})
}

func ok(git Git, args []string, cwd string) (string, error) {
	r, err := git(args, cwd)
	if err != nil {
		return "", err
	}
	if r.Code != 0 {
		detail := strings.TrimSpace(r.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(r.Stdout)
		}
		line, _, _ := strings.Cut(detail, "\n")
		if line == "" {
			line = r.Err
		}
		return "", &GitError{Msg: fmt.Sprintf("git %s failed: %s", args[0], line)}
	}
	return strings.TrimSpace(r.Stdout), nil
}

var taskID = regexp.MustCompile(`^tsk_[0-9A-Z]{26}$`)

// TaskBranch is hilvan/<task>.
func TaskBranch(id string) (string, error) {
	if !taskID.MatchString(id) {
		return "", &GitError{Msg: "invalid task id " + id}
	}
	return "hilvan/" + id, nil
}

// WorktreePath keeps worktrees next to the repository, never inside it:
// <parent>/.hilvan-worktrees/<repo>/<task>.
func WorktreePath(root, id string) (string, error) {
	if _, err := TaskBranch(id); err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(root), ".hilvan-worktrees", filepath.Base(root), id), nil
}

// RepoRoot is the top-level folder of the repository containing cwd.
func RepoRoot(cwd string, git Git) (string, error) {
	out, err := ok(git, []string{"rev-parse", "--show-toplevel"}, cwd)
	return filepath.FromSlash(out), err
}

// CurrentBranch is the checked-out branch of root.
func CurrentBranch(root string, git Git) (string, error) {
	return ok(git, []string{"rev-parse", "--abbrev-ref", "HEAD"}, root)
}

var gitVersion = regexp.MustCompile(`git version (\d+)\.(\d+)`)

// supportsRelativePaths reports git ≥ 2.48, which writes worktree links as relative paths.
// Worktrees created by Windows git are then usable from WSL and vice versa (spike 001).
func supportsRelativePaths(git Git, cwd string) bool {
	r, err := git([]string{"--version"}, cwd)
	if err != nil || r.Code != 0 {
		return false
	}
	m := gitVersion.FindStringSubmatch(r.Stdout)
	if m == nil {
		return false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return major > 2 || (major == 2 && minor >= 48)
}

// CreateWorktree adds the task's worktree on a new branch from base.
func CreateWorktree(root, id, base string, git Git) (path, branch string, err error) {
	if branch, err = TaskBranch(id); err != nil {
		return "", "", err
	}
	if path, err = WorktreePath(root, id); err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", "", err
	}
	args := []string{"worktree", "add"}
	if supportsRelativePaths(git, root) {
		args = append(args, "--relative-paths")
	}
	args = append(args, "-b", branch, "--", path, base)
	_, err = ok(git, args, root)
	return path, branch, err
}

// RemoveWorktree removes the task's worktree and deletes its branch.
func RemoveWorktree(root, id string, git Git) error {
	path, err := WorktreePath(root, id)
	if err != nil {
		return err
	}
	branch, _ := TaskBranch(id)
	if _, err := ok(git, []string{"worktree", "remove", "--force", "--", path}, root); err != nil {
		return err
	}
	_, err = ok(git, []string{"branch", "-D", "--", branch}, root)
	return err
}

// FallbackIdentity returns -c options with a neutral identity when git has none configured
// (CI runners, fresh machines), so committing and merging never fail for that reason.
func FallbackIdentity(dir string, git Git) []string {
	name, _ := git([]string{"config", "user.name"}, dir)
	email, _ := git([]string{"config", "user.email"}, dir)
	if strings.TrimSpace(name.Stdout) != "" && strings.TrimSpace(email.Stdout) != "" {
		return nil
	}
	return []string{"-c", "user.name=Hilvan", "-c", "user.email=hilvan@localhost"}
}

// CommitWorktree commits everything the agent changed on the task branch. Repository hooks are
// skipped (--no-verify): they are untrusted repository code (T10). It returns false when there
// was nothing to commit.
func CommitWorktree(wt, message string, git Git) (bool, error) {
	if _, err := ok(git, []string{"add", "-A"}, wt); err != nil {
		return false, err
	}
	staged, err := git([]string{"diff", "--cached", "--quiet"}, wt)
	if err != nil {
		return false, err
	}
	if staged.Code == 0 {
		return false, nil
	}
	args := append(FallbackIdentity(wt, git), "commit", "--no-verify", "-q", "-m", message)
	_, err = ok(git, args, wt)
	return err == nil, err
}

var numstat = regexp.MustCompile(`^(\d+|-)\t(\d+|-)\t(.*)$`)

// DiffSummary is the numstat of base...branch as Criterio diff signals.
func DiffSummary(root, base, branch string, git Git) (domain.DiffSummary, error) {
	out, err := ok(git, []string{"diff", "--numstat", "-z", base + "..." + branch}, root)
	if err != nil {
		return domain.DiffSummary{}, err
	}
	var parts []string
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	added, deleted := 0, 0
	paths := []string{}
	// -z format: "<added>\t<deleted>\t<path>\0"; renames put old and new paths in the next fields.
	for i := 0; i < len(parts); i++ {
		m := numstat.FindStringSubmatch(parts[i])
		if m == nil {
			continue
		}
		if m[1] != "-" {
			n, _ := strconv.Atoi(m[1])
			added += n
		}
		if m[2] != "-" {
			n, _ := strconv.Atoi(m[2])
			deleted += n
		}
		if m[3] != "" {
			paths = append(paths, m[3])
		} else {
			if i+2 < len(parts) && parts[i+2] != "" {
				paths = append(paths, parts[i+2])
			}
			i += 2
		}
	}
	files := len(paths)
	if len(paths) > 2000 {
		paths = paths[:2000]
	}
	return domain.DiffSummary{FilesChanged: files, LinesAdded: added, LinesDeleted: deleted, Paths: paths}, nil
}
