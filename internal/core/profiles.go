package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/process"
)

// CurrentHost describes where Hilvan runs, detecting WSL from /proc/version.
func CurrentHost() launchers.Host {
	osName := runtime.GOOS
	if osName == "windows" {
		osName = "win32"
	}
	inside := false
	if osName == "linux" {
		if v, err := os.ReadFile("/proc/version"); err == nil {
			inside = strings.Contains(strings.ToLower(string(v)), "microsoft")
		}
	}
	return launchers.Host{OS: osName, InsideWSL: inside, Env: process.CurrentEnv()}
}

// ExpandHome expands a leading ~ for paths on the local host.
func ExpandHome(p, home string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		return filepath.Join(home, p[2:])
	}
	return p
}

// DefaultConfigDir is the isolated directory for a new profile: ~/.hilvan-profiles/<id>.
func DefaultConfigDir(id, home string) string { return filepath.Join(home, ".hilvan-profiles", id) }

// ProfileError is a user-facing problem with the profile registry.
type ProfileError struct{ Msg string }

func (e *ProfileError) Error() string { return e.Msg }

// AddProfile rejects duplicate ids and two profiles sharing a config dir (NFR-008).
func AddProfile(f domain.ProfilesFile, p domain.Profile) (domain.ProfilesFile, error) {
	if _, exists := f.Find(p.ID); exists {
		return f, &ProfileError{Msg: fmt.Sprintf("a profile with id %q already exists", p.ID)}
	}
	for _, q := range f.Profiles {
		sameDest := q.App == p.App && q.Destination.Launcher == p.Destination.Launcher && q.Destination.Distro == p.Destination.Distro
		if !sameDest {
			continue
		}
		clash := false
		if q.ConfigDir == nil || p.ConfigDir == nil {
			clash = q.ConfigDir == nil && p.ConfigDir == nil
		} else {
			clash = filepath.Clean(*q.ConfigDir) == filepath.Clean(*p.ConfigDir)
		}
		if clash {
			dir := "the CLI's default login"
			if p.ConfigDir != nil {
				dir = *p.ConfigDir
			}
			return f, &ProfileError{Msg: fmt.Sprintf("profile %q already uses %s; each profile needs its own config dir", q.ID, dir)}
		}
	}
	f.Profiles = append(append([]domain.Profile{}, f.Profiles...), p)
	return f, nil
}

// RemoveProfile removes a profile by id.
func RemoveProfile(f domain.ProfilesFile, id string) (domain.ProfilesFile, error) {
	out := []domain.Profile{}
	found := false
	for _, p := range f.Profiles {
		if p.ID == id {
			found = true
			continue
		}
		out = append(out, p)
	}
	if !found {
		return f, &ProfileError{Msg: fmt.Sprintf("no profile with id %q", id)}
	}
	f.Profiles = out
	return f, nil
}

// FindProfile returns a profile by id.
func FindProfile(f domain.ProfilesFile, id string) (*domain.Profile, error) {
	p, ok := f.Find(id)
	if !ok {
		return nil, &ProfileError{Msg: fmt.Sprintf("no profile with id %q", id)}
	}
	return p, nil
}

// DescribeDestination is native, windows or wsl:<distro>.
func DescribeDestination(p *domain.Profile) string {
	if p.Destination.Launcher == domain.LauncherWSL {
		return "wsl:" + p.Destination.Distro
	}
	return p.Destination.Launcher
}

// DirFindings are problems with a native config dir that would break isolation (T10).
type DirFindings struct {
	Issues, Warnings []string
}

// ConfigDirIssues inspects a config dir on this host.
func ConfigDirIssues(dir string) DirFindings {
	var f DirFindings
	tmp := os.TempDir()
	if dir == tmp || strings.HasPrefix(dir, tmp+string(filepath.Separator)) || strings.HasPrefix(dir, "/tmp/") {
		f.Issues = append(f.Issues, "config dir is under the temporary directory")
	}
	st, err := os.Stat(dir)
	switch {
	case err != nil:
		f.Issues = append(f.Issues, "config dir does not exist (run `hilvan profile login`)")
	case !st.IsDir():
		f.Issues = append(f.Issues, "config dir is not a directory")
	case runtime.GOOS != "windows" && st.Mode().Perm()&0o077 != 0:
		f.Warnings = append(f.Warnings, fmt.Sprintf("config dir permissions are %o; 700 recommended", st.Mode().Perm()))
	}
	return f
}

// EnsureConfigDir creates a private config dir.
func EnsureConfigDir(dir string) error { return os.MkdirAll(dir, 0o700) }

// ProfileCheck is the verified state of a profile (AC-017).
type ProfileCheck struct {
	ProfileID   string              `json:"profileId"`
	App         string              `json:"app"`
	Destination string              `json:"destination"`
	ConfigDir   *string             `json:"configDir"`
	Auth        adapters.AuthStatus `json:"auth"`
	Issues      []string            `json:"issues"`
	Warnings    []string            `json:"warnings"`
}

// CheckDeps are the collaborators of CheckProfile and DiscoverTools.
type CheckDeps struct {
	Host            launchers.Host
	Run             func(launchers.Invocation) (process.Result, error)
	ConfigDirIssues func(dir string) DirFindings
}

// RunInvocation runs a short command for CheckDeps.
func RunInvocation(inv launchers.Invocation) (process.Result, error) {
	return process.Run(inv.Command, inv.Args, process.Options{Env: inv.Env, Dir: inv.Cwd})
}

var apiMethod = regexp.MustCompile(`(?i)api|console`)

// CheckProfile verifies executable, config dir and active identity of a profile.
func CheckProfile(p *domain.Profile, deps CheckDeps) ProfileCheck {
	adapter, _ := adapters.For(p.App)
	c := ProfileCheck{ProfileID: p.ID, App: p.App, Destination: DescribeDestination(p), ConfigDir: p.ConfigDir, Issues: []string{}, Warnings: []string{}}
	// Directory checks only apply to paths on this host.
	if p.Destination.Launcher == domain.LauncherNative && p.ConfigDir != nil {
		d := deps.ConfigDirIssues(*p.ConfigDir)
		c.Issues = append(c.Issues, d.Issues...)
		c.Warnings = append(c.Warnings, d.Warnings...)
	}
	if p.Auth.Mode == "api-key" && deps.Host.Env[p.Auth.APIKeyEnv] == "" {
		c.Issues = append(c.Issues, "environment variable "+p.Auth.APIKeyEnv+" is not set")
	}
	inv, err := launchers.BuildInvocation(p.Destination, adapter.AuthStatusCommand(p, deps.Host.Env), deps.Host)
	if err != nil {
		c.Issues = append(c.Issues, err.Error())
		return c
	}
	r, err := deps.Run(inv)
	if err != nil {
		c.Issues = append(c.Issues, err.Error())
		return c
	}
	switch r.Err {
	case "not_found":
		c.Issues = append(c.Issues, inv.Command+" not found in PATH")
		return c
	case "timeout":
		c.Issues = append(c.Issues, "auth status timed out")
		return c
	}
	c.Auth = adapter.ParseAuthStatus(r.Stdout, r.Stderr, r.Code)
	if !c.Auth.LoggedIn {
		c.Issues = append(c.Issues, "not logged in (run `hilvan profile login`)")
	}
	if c.Auth.LoggedIn && !c.Auth.Verifiable {
		c.Warnings = append(c.Warnings, "active account cannot be verified")
	}
	if p.Auth.Mode == "subscription" && c.Auth.Method != "" && apiMethod.MatchString(c.Auth.Method) {
		c.Warnings = append(c.Warnings, fmt.Sprintf("profile expects a subscription login but the CLI reports %q", c.Auth.Method))
	}
	return c
}

// SharedAccountWarnings flags two profiles of the same app logged into one account (AC-002).
func SharedAccountWarnings(checks []ProfileCheck) []string {
	var keys []string
	by := map[string][]string{}
	for _, c := range checks {
		if !c.Auth.LoggedIn || c.Auth.Account == "" {
			continue
		}
		k := c.App + "\x00" + strings.ToLower(c.Auth.Account)
		if _, ok := by[k]; !ok {
			keys = append(keys, k)
		}
		by[k] = append(by[k], c.ProfileID)
	}
	out := []string{}
	for _, k := range keys {
		if ids := by[k]; len(ids) > 1 {
			out = append(out, "profiles "+strings.Join(ids, ", ")+" are logged into the same account")
		}
	}
	return out
}

var (
	nonSlug    = regexp.MustCompile(`[^a-z0-9]+`)
	edgeDashes = regexp.MustCompile(`^-+|-+$`)
)

// ToolIDFromName turns a discovered MCP server name into a tool id:
// "claude.ai Jira Cloud" → "claude-ai-jira-cloud".
func ToolIDFromName(name string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(name)) {
		if !unicode.Is(unicode.M, r) {
			b.WriteRune(r)
		}
	}
	slug := edgeDashes.ReplaceAllString(nonSlug.ReplaceAllString(b.String(), "-"), "")
	if len(slug) > 63 {
		slug = slug[:63]
	}
	slug = strings.TrimRight(slug, "-")
	if slug != "" && (slug[0] >= 'a' && slug[0] <= 'z' || slug[0] >= '0' && slug[0] <= '9') {
		return slug
	}
	out := "t-" + slug
	if len(out) > 63 {
		out = out[:63]
	}
	return out
}

// ToolDiscovery compares the account's MCP servers with the profile's declared tools.
type ToolDiscovery struct {
	Discovered []DiscoveredTool `json:"discovered"`
	// Missing are discovered but not declared in provides.
	Missing []string `json:"missing"`
	// Stale are declared in provides but not found in the account.
	Stale []string `json:"stale"`
	Error string   `json:"error,omitempty"`
}

// DiscoveredTool is an MCP server with its tool id.
type DiscoveredTool struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// DiscoverTools lists the account's own MCP servers without calling the model (FR-021). It runs
// from the home directory so repository .mcp.json servers are not mixed in.
func DiscoverTools(p *domain.Profile, deps CheckDeps) ToolDiscovery {
	adapter, _ := adapters.For(p.App)
	cmd := adapter.ListToolsCommand(p, deps.Host.Env)
	cmd.Cwd, _ = os.UserHomeDir()
	inv, err := launchers.BuildInvocation(p.Destination, cmd, deps.Host)
	if err != nil {
		return ToolDiscovery{Discovered: []DiscoveredTool{}, Missing: []string{}, Stale: []string{}, Error: err.Error()}
	}
	r, err := deps.Run(inv)
	if err != nil || r.Code != 0 {
		msg := strings.TrimSpace(r.Stderr)
		if msg == "" {
			msg = strings.TrimSpace(r.Stdout)
		}
		msg, _, _ = strings.Cut(msg, "\n")
		if msg == "" {
			msg = r.Err
		}
		if err != nil {
			msg = err.Error()
		}
		if msg == "" {
			msg = "failed"
		}
		return ToolDiscovery{Discovered: []DiscoveredTool{}, Missing: []string{}, Stale: []string{}, Error: msg}
	}
	d := ToolDiscovery{Discovered: []DiscoveredTool{}, Missing: []string{}, Stale: []string{}}
	seen := map[string]bool{}
	for _, t := range adapter.ParseToolList(r.Stdout) {
		id := ToolIDFromName(t.Name)
		d.Discovered = append(d.Discovered, DiscoveredTool{ID: id, Name: t.Name, Status: t.Status})
		if !seen[id] {
			seen[id] = true
			if !contains(p.Tools.Provides, id) {
				d.Missing = append(d.Missing, id)
			}
		}
	}
	for _, id := range p.Tools.Provides {
		if !seen[id] {
			d.Stale = append(d.Stale, id)
		}
	}
	return d
}

// WSLDiscoveryInvocation finds where a CLI lives inside a distro, and the PATH it needs, with an
// interactive bash: Ubuntu's .bashrc returns early in non-interactive shells, before nvm loads
// (spike 001). The script is a constant; only the CLI name (from the adapter) varies.
func WSLDiscoveryInvocation(distro, cli string, h launchers.Host) launchers.Invocation {
	script := "command -v " + cli + `; printf 'HILVAN_PATH=%s\n' "$PATH"`
	return launchers.Invocation{Command: "wsl.exe", Args: []string{"-d", distro, "--", "bash", "-ic", script}, Env: h.Env}
}

// ParseWSLDiscovery reads the output of WSLDiscoveryInvocation. Banners printed by .bashrc are
// ignored: only an absolute path line and the HILVAN_PATH line count.
func ParseWSLDiscovery(stdout string) (executable, path string, err error) {
	for _, line := range strings.Split(strings.ReplaceAll(stdout, "\r", ""), "\n") {
		switch {
		case strings.HasPrefix(line, "HILVAN_PATH="):
			path = strings.TrimPrefix(line, "HILVAN_PATH=")
		case strings.HasPrefix(line, "/") && !strings.ContainsAny(line, " \t"):
			executable = line
		}
	}
	if executable == "" {
		return "", "", fmt.Errorf("the CLI was not found in the distro (is it installed for this user?)")
	}
	if path == "" {
		return "", "", fmt.Errorf("could not read PATH from the distro")
	}
	return executable, path, nil
}
