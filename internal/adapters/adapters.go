package adapters

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/launchers"
)

// AuthStatus is the identity the CLI reports for a profile; it never contains credentials.
type AuthStatus struct {
	LoggedIn bool `json:"loggedIn"`
	// Method is e.g. claude.ai, console, chatgpt or api-key.
	Method string `json:"method,omitempty"`
	// Account is shown to the user (email or organisation) when the CLI exposes it.
	Account string `json:"account,omitempty"`
	// Verifiable is false when the CLI cannot tell which account is active (Codex).
	Verifiable bool   `json:"verifiable"`
	Detail     string `json:"detail,omitempty"`
}

// DiscoveredTool is an MCP server configured for the account (connected|failed|pending|unknown).
type DiscoveredTool struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// RunSpec describes one non-interactive run.
type RunSpec struct {
	// Cwd is the working directory as the destination sees it.
	Cwd string
	// Mode is write (may edit files in the worktree) or read.
	Mode            string
	ResumeSessionID string
	Model           string // overrides the profile's model
}

// Adapter is one agent application (docs/design/contrato-adaptador.md).
type Adapter interface {
	App() string
	// DefaultExecutable is the CLI name used when the destination does not override it.
	DefaultExecutable() string
	// ConfigDirEnv isolates the profile (CLAUDE_CONFIG_DIR / CODEX_HOME).
	ConfigDirEnv() string
	// ProfileEnv sets the config dir, maps an api-key profile's variable to the provider's and
	// removes stray provider keys, so a subscription profile cannot silently authenticate as
	// someone else (NFR-008).
	ProfileEnv(p *domain.Profile, hostEnv map[string]string) (env map[string]string, unset []string)
	LoginCommand(p *domain.Profile, hostEnv map[string]string) launchers.Command
	AuthStatusCommand(p *domain.Profile, hostEnv map[string]string) launchers.Command
	ParseAuthStatus(stdout, stderr string, exitCode int) AuthStatus
	// RunCommand is a non-interactive run; the prompt goes to stdin (SR-003).
	RunCommand(p *domain.Profile, req RunSpec, hostEnv map[string]string) launchers.Command
	// ListToolsCommand lists the account's MCP servers without calling the model.
	ListToolsCommand(p *domain.Profile, hostEnv map[string]string) launchers.Command
	ParseToolList(stdout string) []DiscoveredTool
	// NewParser returns a fresh stateful line parser for one run.
	NewParser() func(line string) []Event
}

// For returns the adapter of an application.
func For(app string) (Adapter, bool) {
	switch app {
	case domain.AppClaudeCode:
		return claude{}, true
	case domain.AppCodex:
		return codex{}, true
	}
	return nil, false
}

func profileEnv(p *domain.Profile, hostEnv map[string]string, dirVar, keyVar string, providerKeys []string) (map[string]string, []string) {
	env := map[string]string{}
	var unset []string
	if p.ConfigDir != nil {
		env[dirVar] = *p.ConfigDir
	} else {
		unset = append(unset, dirVar)
	}
	if p.Auth.Mode == "api-key" {
		if v := hostEnv[p.Auth.APIKeyEnv]; v != "" {
			env[keyVar] = v
		}
		for _, k := range providerKeys {
			if k != keyVar {
				unset = append(unset, k)
			}
		}
		return env, unset
	}
	return env, append(unset, providerKeys...)
}

func command(exe string, args []string, env map[string]string, unset []string, cwd string) launchers.Command {
	return launchers.Command{Executable: exe, Args: args, Env: env, UnsetEnv: unset, Cwd: cwd}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return strings.TrimSuffix(line, "\r")
}

// ── Claude Code ────────────────────────────────────────────────────────────

type claude struct{}

var claudeKeys = []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"}

func (claude) App() string               { return domain.AppClaudeCode }
func (claude) DefaultExecutable() string { return "claude" }
func (claude) ConfigDirEnv() string      { return "CLAUDE_CONFIG_DIR" }

func (claude) ProfileEnv(p *domain.Profile, hostEnv map[string]string) (map[string]string, []string) {
	return profileEnv(p, hostEnv, "CLAUDE_CONFIG_DIR", "ANTHROPIC_API_KEY", claudeKeys)
}

func (c claude) cmd(p *domain.Profile, hostEnv map[string]string, cwd string, args ...string) launchers.Command {
	env, unset := c.ProfileEnv(p, hostEnv)
	return command("claude", args, env, unset, cwd)
}

func (c claude) LoginCommand(p *domain.Profile, h map[string]string) launchers.Command {
	return c.cmd(p, h, "", "auth", "login")
}

func (c claude) AuthStatusCommand(p *domain.Profile, h map[string]string) launchers.Command {
	return c.cmd(p, h, "", "auth", "status", "--json")
}

// RunCommand never loads repository settings, hooks or .mcp.json (ADR-0013, verified:
// --strict-mcp-config alone still runs repository hooks); MCP servers follow the profile mode.
func (c claude) RunCommand(p *domain.Profile, req RunSpec, h map[string]string) launchers.Command {
	args := []string{"-p", "--output-format", "stream-json", "--verbose", "--setting-sources", "user"}
	if p.Tools.MCP != "account" {
		args = append(args, "--strict-mcp-config")
	}
	if p.Tools.MCP == "profile" && p.Tools.MCPConfig != "" {
		args = append(args, "--mcp-config", p.Tools.MCPConfig)
	}
	mode := "dontAsk"
	if req.Mode == "write" {
		mode = "acceptEdits"
	}
	args = append(args, "--permission-prompts", "none", "--permission-mode", mode)
	if model := modelFor(p, req); model != "" {
		args = append(args, "--model", model)
	}
	if req.ResumeSessionID != "" {
		args = append(args, "--resume", req.ResumeSessionID)
	}
	if req.Mode == "read" {
		// D-031 and SR-037: an explicit list of read-only tools, one argument per tool, last so that
		// the variadic list cannot swallow another flag. Everything else stays denied under dontAsk.
		args = append(append(args, "--allowedTools"), ReadOnlyTools...)
	}
	return c.cmd(p, h, req.Cwd, args...)
}

// ReadOnlyTools are the tools a read-only Claude run may use (D-031, SR-037). Git subcommands that
// can write (git log --output, git diff --output) are left out on purpose.
var ReadOnlyTools = []string{"Read", "Glob", "Grep", "Bash(ls:*)", "Bash(git ls-files:*)"}

func modelFor(p *domain.Profile, req RunSpec) string {
	if req.Model != "" {
		return req.Model
	}
	if p.Model != nil {
		return *p.Model
	}
	return ""
}

func (c claude) ListToolsCommand(p *domain.Profile, h map[string]string) launchers.Command {
	return c.cmd(p, h, "", "mcp", "list")
}

var (
	claudeToolLine = regexp.MustCompile(`^(.+?):\s.*\s-\s(\S+)\s*(.*)$`)
	checkingMCP    = regexp.MustCompile(`(?i)checking mcp`)
	connected      = regexp.MustCompile(`connected|✔|✓`)
	failedTool     = regexp.MustCompile(`fail|✗|error`)
	pendingTool    = regexp.MustCompile(`pending|⏸`)
)

// ParseToolList reads lines like `name: <target> - ✔ Connected`. The target is not kept: it
// may embed tokens.
func (claude) ParseToolList(stdout string) []DiscoveredTool {
	out := []DiscoveredTool{}
	for _, raw := range regexp.MustCompile(`\r?\n`).Split(stdout, -1) {
		m := claudeToolLine.FindStringSubmatch(strings.TrimSpace(raw))
		if m == nil || m[1] == "" || checkingMCP.MatchString(raw) {
			continue
		}
		st := strings.ToLower(m[2] + " " + m[3])
		status := "unknown"
		switch {
		case connected.MatchString(st):
			status = "connected"
		case failedTool.MatchString(st):
			status = "failed"
		case pendingTool.MatchString(st):
			status = "pending"
		}
		out = append(out, DiscoveredTool{Name: strings.TrimSpace(m[1]), Status: status})
	}
	return out
}

func (claude) ParseAuthStatus(stdout, stderr string, exitCode int) AuthStatus {
	var v any
	if json.Unmarshal([]byte(stdout), &v) != nil {
		src := stderr // (stderr || stdout).trim().split(/\r?\n/)[0]
		if src == "" {
			src = stdout
		}
		return AuthStatus{Verifiable: true, Detail: "unreadable auth status: " + firstLine(src)}
	}
	data := obj(v)
	method, _ := strOf(data["authMethod"])
	email, _ := strOf(data["email"])
	org, _ := strOf(data["orgName"])
	s := AuthStatus{LoggedIn: data["loggedIn"] == true, Verifiable: true, Method: method, Account: email}
	if s.Account == "" {
		s.Account = org
	}
	if org != "" && email != "" && !strings.Contains(org, email) {
		s.Detail = "org: " + org
	}
	return s
}

func (claude) NewParser() func(string) []Event { return ParseClaudeLine }

// ── Codex ──────────────────────────────────────────────────────────────────

type codex struct{}

var codexKeys = []string{"OPENAI_API_KEY", "CODEX_API_KEY"}

func (codex) App() string               { return domain.AppCodex }
func (codex) DefaultExecutable() string { return "codex" }
func (codex) ConfigDirEnv() string      { return "CODEX_HOME" }

func (codex) ProfileEnv(p *domain.Profile, hostEnv map[string]string) (map[string]string, []string) {
	return profileEnv(p, hostEnv, "CODEX_HOME", "CODEX_API_KEY", codexKeys)
}

func (c codex) cmd(p *domain.Profile, hostEnv map[string]string, cwd string, args ...string) launchers.Command {
	env, unset := c.ProfileEnv(p, hostEnv)
	return command("codex", args, env, unset, cwd)
}

func (c codex) LoginCommand(p *domain.Profile, h map[string]string) launchers.Command {
	return c.cmd(p, h, "", "login")
}

func (c codex) AuthStatusCommand(p *domain.Profile, h map[string]string) launchers.Command {
	return c.cmd(p, h, "", "login", "status")
}

func (c codex) RunCommand(p *domain.Profile, req RunSpec, h map[string]string) launchers.Command {
	sandbox := "read-only"
	if req.Mode == "write" {
		sandbox = "workspace-write"
	}
	common := []string{"--json", "--cd", req.Cwd, "--sandbox", sandbox}
	if model := modelFor(p, req); model != "" {
		common = append(common, "--model", model)
	}
	args := []string{"exec"}
	if req.ResumeSessionID != "" {
		args = append(args, "resume", req.ResumeSessionID)
	}
	args = append(append(args, common...), "-")
	return c.cmd(p, h, req.Cwd, args...)
}

func (c codex) ListToolsCommand(p *domain.Profile, h map[string]string) launchers.Command {
	return c.cmd(p, h, "", "mcp", "list", "--json")
}

func (codex) ParseToolList(stdout string) []DiscoveredTool {
	var list []map[string]any
	if json.Unmarshal([]byte(stdout), &list) != nil {
		return []DiscoveredTool{}
	}
	out := []DiscoveredTool{}
	for _, x := range list {
		name, ok := strOf(x["name"])
		if !ok {
			continue
		}
		status := "unknown"
		if x["enabled"] == false {
			status = "failed"
		}
		out = append(out, DiscoveredTool{Name: name, Status: status})
	}
	return out
}

var (
	loggedInLine = regexp.MustCompile(`(?i)logged in`)
	notLoggedIn  = regexp.MustCompile(`(?i)not logged in`)
	chatgpt      = regexp.MustCompile(`(?i)chatgpt`)
	apiKey       = regexp.MustCompile(`(?i)api key`)
	errorLine    = regexp.MustCompile(`(?i)error`)
	lineBreak    = regexp.MustCompile(`\r?\n`)
)

// ParseAuthStatus reads `codex login status`, which prints text and does not identify the
// account (spike 002).
func (codex) ParseAuthStatus(stdout, stderr string, exitCode int) AuthStatus {
	lines := lineBreak.Split(stdout+"\n"+stderr, -1)
	find := func(re *regexp.Regexp) string {
		for _, l := range lines {
			if re.MatchString(l) {
				return strings.TrimSpace(l)
			}
		}
		return ""
	}
	line := find(loggedInLine)
	if line != "" && !notLoggedIn.MatchString(line) {
		s := AuthStatus{LoggedIn: true, Detail: "Codex does not report which account is active"}
		switch {
		case chatgpt.MatchString(line):
			s.Method = "chatgpt"
		case apiKey.MatchString(line):
			s.Method = "api-key"
		}
		return s
	}
	detail := line
	if detail == "" {
		detail = find(errorLine)
	}
	if detail == "" {
		detail = "exit " + itoa(exitCode)
	}
	return AuthStatus{Detail: detail}
}

func (codex) NewParser() func(string) []Event {
	p := &CodexParser{}
	return p.Parse
}

func itoa(i int) string { return strconv.Itoa(i) }
