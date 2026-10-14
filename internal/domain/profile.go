package domain

import (
	"regexp"
	"sort"
	"strings"
)

// App is an agent application with an adapter (D-015).
type App = string

const (
	AppClaudeCode App = "claude-code"
	AppCodex      App = "codex"
)

// Launcher says how a command reaches its execution environment (ADR-0005).
type Launcher = string

const (
	LauncherNative  Launcher = "native"
	LauncherWSL     Launcher = "wsl"
	LauncherWindows Launcher = "windows"
)

// Destination is where a profile's agent runs.
type Destination struct {
	Launcher Launcher `json:"launcher" yaml:"launcher"`
	// Distro is the WSL distribution, required when Launcher is wsl.
	Distro string `json:"distro,omitempty" yaml:"distro,omitempty"`
	// Executable overrides the app's CLI name. For wsl it is an absolute path inside the distro,
	// discovered when the profile is created (wsl.exe does not load the user's shell PATH).
	Executable string `json:"executable,omitempty" yaml:"executable,omitempty"`
	// Path is the PATH used inside the destination (wsl), so CLIs installed with nvm or in
	// ~/.local/bin and their node are found (spike 001).
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

// Auth is the authentication mode (ADR-0006). Hilvan never stores credentials:
// subscription relies on the official CLI login inside ConfigDir; api-key names
// an environment variable.
type Auth struct {
	Mode      string `json:"mode" yaml:"mode"`
	APIKeyEnv string `json:"apiKeyEnv,omitempty" yaml:"apiKeyEnv,omitempty"`
}

// Tools describes the MCP servers and capabilities of a profile (ADR-0013, FR-021).
type Tools struct {
	// MCP is account (the account's own servers, default), profile (only MCPConfig) or none.
	MCP       string `json:"mcp" yaml:"mcp"`
	MCPConfig string `json:"mcpConfig,omitempty" yaml:"mcpConfig,omitempty"`
	// Provides lists capabilities declared for eligibility checks (FR-006, FR-011).
	Provides []string `json:"provides" yaml:"provides"`
	// Aliases are words that reveal a task needs a tool, e.g. jira: [ticket, incidencia].
	Aliases Aliases `json:"aliases" yaml:"aliases"`
	// Web allows web search and fetch in read-only runs (SR-038). Nil means allowed.
	Web *bool `json:"web,omitempty" yaml:"web,omitempty"`
	// WebClassifications are the task classifications that may use web tools, because search
	// queries leave the machine. Empty means public and personal only.
	WebClassifications []string `json:"webClassifications,omitempty" yaml:"webClassifications,omitempty"`
}

// DefaultWebClassifications are the classifications whose read-only runs may use web tools (SR-038).
var DefaultWebClassifications = []string{"public", "personal"}

// WebFor reports whether a read-only run of a task with this classification may use web tools.
func (p *Profile) WebFor(classification string) bool {
	if p.Tools.Web != nil && !*p.Tools.Web {
		return false
	}
	allowed := p.Tools.WebClassifications
	if len(allowed) == 0 {
		allowed = DefaultWebClassifications
	}
	for _, c := range allowed {
		if c == classification {
			return true
		}
	}
	return false
}

// Aliases maps a tool id to the words that reveal it is needed.
type Aliases map[string][]string

// Keys returns the tool ids in sorted order (map iteration is random in Go).
func (a Aliases) Keys() []string {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Profile is one account in one environment.
type Profile struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
	App   App    `json:"app" yaml:"app"`
	Auth  Auth   `json:"auth" yaml:"auth"`
	// ConfigDir is CLAUDE_CONFIG_DIR / CODEX_HOME inside the destination environment.
	// nil means the CLI's default login: the variable is removed, not set, because
	// Claude Code keeps the default account outside ~/.claude (in ~/.claude.json).
	ConfigDir   *string     `json:"configDir" yaml:"configDir"`
	Destination Destination `json:"destination" yaml:"destination"`
	// Accepts lists the data classifications this profile may receive (D-022).
	Accepts []string `json:"accepts" yaml:"accepts"`
	Tags    []string `json:"tags" yaml:"tags"`
	Tools   Tools    `json:"tools" yaml:"tools"`
	// Routing is default or reserved: reserved profiles only receive tasks that need their tools.
	Routing    string  `json:"routing" yaml:"routing"`
	Model      *string `json:"model" yaml:"model"`
	Preference int     `json:"preference" yaml:"preference"`
}

// ProfilesFile is the content of profiles.yaml.
type ProfilesFile struct {
	Version  int       `json:"version" yaml:"version"`
	Profiles []Profile `json:"profiles" yaml:"profiles"`
}

var envVarName = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// Normalize fills the defaults that zod applies (empty lists, account MCP, default routing).
func (p *Profile) Normalize() {
	if p.Tags == nil {
		p.Tags = []string{}
	}
	if p.Tools.MCP == "" {
		p.Tools.MCP = "account"
	}
	if p.Tools.Provides == nil {
		p.Tools.Provides = []string{}
	}
	if p.Tools.Aliases == nil {
		p.Tools.Aliases = Aliases{}
	}
	if p.Routing == "" {
		p.Routing = "default"
	}
	if p.Auth.Mode == "subscription" {
		p.Auth.APIKeyEnv = "" // zod strips keys that the subscription variant does not declare
	}
}

// Normalize fills defaults in every profile.
func (f *ProfilesFile) Normalize() {
	if f.Profiles == nil {
		f.Profiles = []Profile{}
	}
	for i := range f.Profiles {
		f.Profiles[i].Normalize()
	}
}

func (p *Profile) validate(b *issues, path string) {
	b.id(field(path, "id"), p.ID)
	b.nonEmpty(field(path, "label"), p.Label)
	b.oneOf(field(path, "app"), p.App, AppClaudeCode, AppCodex)
	switch p.Auth.Mode {
	case "subscription":
	case "api-key":
		if !envVarName.MatchString(p.Auth.APIKeyEnv) {
			b.add(field(path, "auth.apiKeyEnv"), "must be an environment variable name")
		}
	default:
		b.add(field(path, "auth.mode"), `Invalid input: expected one of "subscription"|"api-key"`)
	}
	if p.ConfigDir != nil && *p.ConfigDir == "" {
		b.nonEmpty(field(path, "configDir"), "")
	}
	dest := field(path, "destination")
	b.oneOf(field(dest, "launcher"), p.Destination.Launcher, LauncherNative, LauncherWSL, LauncherWindows)
	if p.Destination.Launcher == LauncherWSL && p.Destination.Distro == "" {
		b.add(field(dest, "distro"), "required when launcher is wsl")
	}
	for _, f := range []struct{ name, v string }{{"distro", p.Destination.Distro}, {"executable", p.Destination.Executable}, {"path", p.Destination.Path}} {
		if strings.ContainsAny(f.v, "\x00\r\n") {
			b.add(field(dest, f.name), "must be a single line")
		}
	}
	if len(p.Accepts) == 0 {
		b.add(field(path, "accepts"), "Too small: expected array to have >=1 items")
	}
	b.nonEmptyAll(field(path, "accepts"), p.Accepts)
	b.ids(field(path, "tags"), p.Tags)
	tools := field(path, "tools")
	b.oneOf(field(tools, "mcp"), p.Tools.MCP, "account", "profile", "none")
	if p.Tools.MCP == "profile" && p.Tools.MCPConfig == "" {
		b.add(field(tools, "mcpConfig"), `required when mcp is "profile"`)
	}
	b.ids(field(tools, "provides"), p.Tools.Provides)
	for _, k := range p.Tools.Aliases.Keys() {
		b.id(field(tools, "aliases"), k)
		words := p.Tools.Aliases[k]
		if len(words) > 50 {
			b.add(field(field(tools, "aliases"), k), "Too big: expected array to have <=50 items")
		}
		for i, w := range words {
			if n := utf16Len(w); n < 2 || n > 60 {
				b.add(index(field(field(tools, "aliases"), k), i), "alias must have 2 to 60 characters")
			}
		}
	}
	b.oneOf(field(path, "routing"), p.Routing, "default", "reserved")
	if p.Model != nil && *p.Model == "" {
		b.nonEmpty(field(path, "model"), "")
	}
}

// Validate checks a normalized profile.
func (p *Profile) Validate() Issues {
	var b issues
	p.validate(&b, "")
	return b.list
}

// Validate checks a normalized profiles file, including unique ids.
func (f *ProfilesFile) Validate() Issues {
	var b issues
	if f.Version != 1 {
		b.add("version", "Invalid input: expected 1")
	}
	seen := map[string]bool{}
	for i := range f.Profiles {
		p := &f.Profiles[i]
		if seen[p.ID] {
			b.add(field(index("profiles", i), "id"), "duplicate id %q", p.ID)
		}
		seen[p.ID] = true
		p.validate(&b, index("profiles", i))
	}
	return b.list
}

// Find returns the profile with the given id.
func (f *ProfilesFile) Find(id string) (*Profile, bool) {
	for i := range f.Profiles {
		if f.Profiles[i].ID == id {
			return &f.Profiles[i], true
		}
	}
	return nil, false
}

// utf16Len counts UTF-16 code units, which is what JavaScript's String.length reports.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
