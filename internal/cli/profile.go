package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/process"
)

// ParseAliases turns repeated "--alias jira=ticket,incidencia" into {jira: [ticket incidencia]}.
func ParseAliases(items []string) (domain.Aliases, error) {
	out := domain.Aliases{}
	for _, item := range items {
		tool, words, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(tool) == "" || words == "" {
			return nil, fmt.Errorf("--alias must look like tool=word1,word2 (got %q)", item)
		}
		t := strings.TrimSpace(tool)
		out[t] = append(out[t], SplitList(words)...)
	}
	return out, nil
}

func defaultDir(app string) string {
	if app == domain.AppCodex {
		return ".codex"
	}
	return ".claude"
}

// dirLabel is a human label for a profile's config dir.
func dirLabel(p *domain.Profile) string {
	if p.ConfigDir != nil {
		return *p.ConfigDir
	}
	return "CLI default login (~/" + defaultDir(p.App) + ")"
}

// BuildProfile turns add options into a validated profile.
func BuildProfile(c *Ctx, id, app string, accepts []string, useDefaultLogin bool) (domain.Profile, error) {
	if app != domain.AppClaudeCode && app != domain.AppCodex {
		return domain.Profile{}, fmt.Errorf(`✖ Invalid option: expected one of "claude-code"|"codex"`)
	}
	launcher := c.Str("launcher")
	var configDir *string
	if v := c.Str("config-dir"); v != "" {
		configDir = &v
	} else {
		if launcher != domain.LauncherNative {
			return domain.Profile{}, errors.New("--config-dir is required for non-native destinations (path in that environment)")
		}
		if !useDefaultLogin {
			home, _ := os.UserHomeDir()
			d := core.DefaultConfigDir(id, home)
			configDir = &d
		}
	}
	if launcher == domain.LauncherNative && configDir != nil {
		home, _ := os.UserHomeDir()
		d := core.ExpandHome(*configDir, home)
		configDir = &d
	}
	auth := domain.Auth{Mode: "subscription"}
	if c.Str("auth") == "api-key" {
		auth = domain.Auth{Mode: "api-key", APIKeyEnv: c.Str("api-key-env")}
	}
	aliases, err := ParseAliases(c.All("alias"))
	if err != nil {
		return domain.Profile{}, err
	}
	mcp := c.Str("mcp")
	if mcp == "" {
		mcp = "account"
		if c.Str("mcp-config") != "" {
			mcp = "profile"
		}
	}
	pref := 0
	if v := c.Str("preference"); v != "" {
		if pref, err = strconv.Atoi(v); err != nil {
			return domain.Profile{}, errors.New("✖ Invalid input: expected int, received number\n  → at preference")
		}
	}
	p := domain.Profile{
		ID: id, Label: or(c.Str("label"), id), App: app, Auth: auth, ConfigDir: configDir,
		Destination: domain.Destination{Launcher: launcher, Distro: c.Str("distro"), Executable: c.Str("executable")},
		Accepts:     accepts, Tags: c.List("tags"),
		Tools:   domain.Tools{MCP: mcp, MCPConfig: c.Str("mcp-config"), Provides: c.List("provides"), Aliases: aliases},
		Routing: or(c.Str("routing"), "default"), Preference: pref,
	}
	if m := c.Str("model"); m != "" {
		p.Model = &m
	}
	p.Normalize()
	if issues := p.Validate(); len(issues) > 0 {
		return p, errors.New(issues.Pretty())
	}
	return p, nil
}

// discoverWSL fills the executable and PATH of a wsl profile from the distro (spike 001).
func discoverWSL(c *Ctx, p *domain.Profile) error {
	if p.Destination.Launcher != domain.LauncherWSL || c.App.Host.OS != "win32" {
		return nil
	}
	adapter, _ := adapters.For(p.App)
	inv := core.WSLDiscoveryInvocation(p.Destination.Distro, adapter.DefaultExecutable(), c.App.Host)
	r, err := process.Run(inv.Command, inv.Args, process.Options{Env: inv.Env, Timeout: 60 * time.Second})
	if err != nil {
		return err
	}
	exe, path, err := core.ParseWSLDiscovery(r.Stdout)
	if err != nil {
		return fmt.Errorf("%s in %s: %v", adapter.DefaultExecutable(), p.Destination.Distro, err)
	}
	if p.Destination.Executable == "" {
		p.Destination.Executable = exe
	}
	p.Destination.Path = path
	c.Println("  found " + p.Destination.Executable + " in " + p.Destination.Distro)
	return nil
}

func (c *Ctx) checkDeps() core.CheckDeps {
	return core.CheckDeps{
		Host: c.App.Host,
		Run: func(inv launchers.Invocation) (process.Result, error) {
			return process.Run(inv.Command, inv.Args, process.Options{Env: inv.Env, Dir: inv.Cwd, Timeout: 20 * time.Second})
		},
		ConfigDirIssues: core.ConfigDirIssues,
	}
}

func formatCheck(ch core.ProfileCheck) string {
	icon := "✔"
	if len(ch.Issues) > 0 {
		icon = "✖"
	} else if len(ch.Warnings) > 0 {
		icon = "!"
	}
	who := "not logged in"
	if ch.Auth.LoggedIn {
		who = or(ch.Auth.Account, "account not reported")
		if ch.Auth.Method != "" {
			who += " (" + ch.Auth.Method + ")"
		}
	}
	lines := []string{fmt.Sprintf("%s %s  [%s @ %s]  %s", icon, ch.ProfileID, ch.App, ch.Destination, who)}
	for _, i := range ch.Issues {
		lines = append(lines, "    ✖ "+i)
	}
	for _, w := range ch.Warnings {
		lines = append(lines, "    ! "+w)
	}
	return strings.Join(lines, "\n")
}

func (c *Ctx) loadProfiles() (domain.ProfilesFile, error) {
	return config.LoadProfiles(c.App.profilesFile())
}

func (c *Ctx) saveProfiles(f domain.ProfilesFile) error {
	return config.SaveProfiles(c.App.profilesFile(), f)
}

func replaceProfile(f domain.ProfilesFile, p domain.Profile) domain.ProfilesFile {
	out := make([]domain.Profile, len(f.Profiles))
	for i, q := range f.Profiles {
		if q.ID == p.ID {
			q = p
		}
		out[i] = q
	}
	f.Profiles = out
	return f
}

func promptMissing(c *Ctx, id, app *string, accepts *[]string, useDefault *bool) bool {
	a := c.App
	fmt.Fprintln(a.Stdout, "┌  New Hilvan profile")
	if *app == "" {
		v, ok := a.Select("Agent application", []Choice{{Value: "claude-code", Label: "Claude Code"}, {Value: "codex", Label: "Codex CLI"}})
		if !ok {
			return false
		}
		*app = v
	}
	if *id == "" {
		v, ok := a.Text("Profile id (lowercase, e.g. claude-personal)", "", func(v string) string {
			if domain.IDPattern.MatchString(v) {
				return ""
			}
			return "Invalid id"
		})
		if !ok {
			return false
		}
		*id = v
	}
	if !c.Has("label") {
		v, ok := a.Text("Label", *id, nil)
		if !ok {
			return false
		}
		c.values["label"] = []string{v}
	}
	if *accepts == nil {
		var choices []Choice
		for _, cl := range DefaultClassifications {
			choices = append(choices, Choice{Value: cl, Label: cl})
		}
		v, ok := a.MultiSelect("Data classifications this profile may receive", choices, nil, true)
		if !ok {
			return false
		}
		*accepts = v
	}
	if !c.Has("config-dir") && !*useDefault {
		*useDefault = a.Confirm("Reuse the existing login in ~/"+defaultDir(*app)+"? (No = new isolated directory)", false)
	}
	return true
}

func profileCommand() *Command {
	add := &Command{
		Name: "add",
		Desc: "Register a profile. Prompts for missing values when run in a terminal.",
		Options: []*Option{
			{Flags: "--id <id>", Desc: "stable id, e.g. claude-personal"},
			{Flags: "--app <app>", Desc: "claude-code | codex"},
			{Flags: "--label <label>", Desc: "display name"},
			{Flags: "--accepts <list>", Desc: "classifications it may receive (" + strings.Join(DefaultClassifications, ",") + ")"},
			{Flags: "--tags <list>", Desc: "tags for policy rules, e.g. personal"},
			{Flags: "--launcher <launcher>", Desc: "native | wsl | windows", Default: "native"},
			{Flags: "--distro <name>", Desc: "WSL distribution (wsl launcher)"},
			{Flags: "--executable <path>", Desc: "override the CLI executable (wsl: found automatically)"},
			{Flags: "--config-dir <dir>", Desc: "isolated CLI config dir (default ~/.hilvan-profiles/<id>)"},
			{Flags: "--use-default-login", Desc: "reuse the CLI's default directory (~/.claude or ~/.codex)"},
			{Flags: "--auth <mode>", Desc: "subscription | api-key", Default: "subscription"},
			{Flags: "--api-key-env <VAR>", Desc: "environment variable holding the API key (api-key mode)"},
			{Flags: "--mcp <mode>", Desc: "account (its own MCP servers, default) | profile (only --mcp-config) | none"},
			{Flags: "--mcp-config <file>", Desc: "MCP servers for this profile (implies --mcp profile)"},
			{Flags: "--routing <role>", Desc: "default | reserved (only for tasks that need its tools)"},
			{Flags: "--alias <tool=words>", Desc: "words that reveal a tool is needed, e.g. jira=ticket,incidencia (repeatable)", Collect: true},
			{Flags: "--provides <list>", Desc: "declared capabilities/tools, e.g. jira,confluence"},
			{Flags: "--model <model>", Desc: "default model"},
			{Flags: "--preference <n>", Desc: "tie-breaker among eligible profiles (higher first)"},
		},
		Run: func(c *Ctx) error {
			id, app, accepts, useDefault := c.Str("id"), c.Str("app"), c.List("accepts"), c.Bool("use-default-login")
			prompted := false
			if (id == "" || app == "" || accepts == nil) && c.App.Interactive {
				if !promptMissing(c, &id, &app, &accepts, &useDefault) {
					return errors.New("Cancelled.")
				}
				prompted = true
			}
			if id == "" || app == "" || len(accepts) == 0 {
				return errors.New("--id, --app and --accepts are required in non-interactive mode")
			}
			p, err := BuildProfile(c, id, app, accepts, useDefault)
			if err != nil {
				return err
			}
			if err := discoverWSL(c, &p); err != nil {
				return err
			}
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			if f, err = core.AddProfile(f, p); err != nil {
				return err
			}
			if err := c.saveProfiles(f); err != nil {
				return err
			}
			c.Println("✔ profile " + p.ID + " added (" + p.App + " @ " + core.DescribeDestination(&p) + ")")
			c.Println("  config dir: " + dirLabel(&p))
			c.Println("  next: hilvan profile login " + p.ID + "   then: hilvan profile check " + p.ID)
			if prompted {
				c.Println("└  Done.")
			}
			return nil
		},
	}
	set := &Command{
		Name: "set",
		Args: []string{"<id>"},
		Desc: "Change routing, MCP mode, tools or aliases of an existing profile.",
		Options: []*Option{
			{Flags: "--routing <role>", Desc: "default | reserved"},
			{Flags: "--mcp <mode>", Desc: "account | profile | none"},
			{Flags: "--mcp-config <file>", Desc: "MCP servers file for --mcp profile"},
			{Flags: "--provides <list>", Desc: "replace the declared tools, e.g. jira,confluence"},
			{Flags: "--alias <tool=words>", Desc: "add aliases, e.g. jira=ticket,incidencia (repeatable)", Collect: true},
			{Flags: "--accepts <list>", Desc: "replace the accepted classifications"},
			{Flags: "--preference <n>", Desc: "tie-breaker among eligible profiles"},
			{Flags: "--rediscover", Desc: "wsl: find the CLI and its PATH in the distro again (e.g. after a Node upgrade)"},
		},
		Run: func(c *Ctx) error {
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			cur, err := core.FindProfile(f, c.Args[0])
			if err != nil {
				return err
			}
			next := *cur
			next.Tools.Aliases = domain.Aliases{}
			for k, v := range cur.Tools.Aliases {
				next.Tools.Aliases[k] = append([]string{}, v...)
			}
			add, err := ParseAliases(c.All("alias"))
			if err != nil {
				return err
			}
			for _, tool := range add.Keys() {
				for _, w := range add[tool] {
					if !contains(next.Tools.Aliases[tool], w) {
						next.Tools.Aliases[tool] = append(next.Tools.Aliases[tool], w)
					}
				}
			}
			if v := c.Str("routing"); v != "" {
				next.Routing = v
			}
			if v := c.List("accepts"); v != nil {
				next.Accepts = v
			}
			if v := c.Str("preference"); v != "" {
				if next.Preference, err = strconv.Atoi(v); err != nil {
					return errors.New("✖ Invalid input: expected int, received number\n  → at preference")
				}
			}
			if v := c.Str("mcp"); v != "" {
				next.Tools.MCP = v
			}
			if v := c.Str("mcp-config"); v != "" {
				next.Tools.MCPConfig = v
			}
			if v := c.List("provides"); v != nil {
				next.Tools.Provides = v
			}
			if c.Bool("rediscover") {
				next.Destination.Executable = ""
				if err := discoverWSL(c, &next); err != nil {
					return err
				}
			}
			if issues := next.Validate(); len(issues) > 0 {
				return errors.New(issues.Pretty())
			}
			if err := c.saveProfiles(replaceProfile(f, next)); err != nil {
				return err
			}
			c.Println(fmt.Sprintf("✔ %s: routing %s · mcp %s · provides %s", next.ID, next.Routing, next.Tools.MCP, or(strings.Join(next.Tools.Provides, ", "), "—")))
			return nil
		},
	}
	tools := &Command{
		Name:    "tools",
		Args:    []string{"<id>"},
		Desc:    "List the MCP servers the account itself has configured and compare with 'provides'.",
		Options: []*Option{{Flags: "--apply", Desc: "add discovered servers to the profile's provides"}},
		Run: func(c *Ctx) error {
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			p, err := core.FindProfile(f, c.Args[0])
			if err != nil {
				return err
			}
			d := core.DiscoverTools(p, c.checkDeps())
			if d.Error != "" {
				return errors.New("could not list MCP servers: " + d.Error)
			}
			if len(d.Discovered) == 0 {
				c.Println(p.ID + ": no MCP servers configured in this account")
			}
			for _, t := range d.Discovered {
				mark := "+"
				if contains(p.Tools.Provides, t.ID) {
					mark = "✔"
				}
				c.Println(fmt.Sprintf("%s %s  (%s, %s)", mark, t.ID, clean(t.Name, 80), t.Status))
			}
			for _, s := range d.Stale {
				c.Println("? " + s + "  declared in provides but not configured in the account")
			}
			if p.Tools.MCP != "account" && len(d.Discovered) > 0 {
				c.Println(`! mcp mode is "` + p.Tools.MCP + `": these account servers are NOT loaded in Hilvan runs`)
			}
			if c.Bool("apply") && len(d.Missing) > 0 {
				next := *p
				next.Tools.Provides = append(append([]string{}, p.Tools.Provides...), d.Missing...)
				if err := c.saveProfiles(replaceProfile(f, next)); err != nil {
					return err
				}
				c.Println("✔ added to provides: " + strings.Join(d.Missing, ", "))
			} else if len(d.Missing) > 0 {
				c.Println("  next: hilvan profile tools " + p.ID + " --apply   (then: hilvan profile set " + p.ID + " --routing reserved)")
			}
			return nil
		},
	}
	list := &Command{
		Name:    "list",
		Desc:    "List configured profiles.",
		Options: []*Option{{Flags: "--json", Desc: "print as JSON"}},
		Run: func(c *Ctx) error {
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			if c.Bool("json") {
				return printJSON(c, f.Profiles)
			}
			if len(f.Profiles) == 0 {
				c.Println("No profiles yet. Add one with: hilvan profile add")
				return nil
			}
			rows := [][]string{{"ID", "APP", "DESTINATION", "AUTH", "ACCEPTS", "CONFIG DIR"}}
			for i := range f.Profiles {
				p := &f.Profiles[i]
				rows = append(rows, []string{p.ID, p.App, core.DescribeDestination(p), p.Auth.Mode, strings.Join(p.Accepts, ","), dirLabel(p)})
			}
			widths := make([]int, 6)
			for _, r := range rows {
				for i, cell := range r {
					widths[i] = max(widths[i], len([]rune(cell)))
				}
			}
			for _, r := range rows {
				cells := make([]string, len(r))
				for i, cell := range r {
					cells[i] = padEnd(cell, widths[i])
				}
				c.Println(strings.TrimRight(strings.Join(cells, "  "), " "))
			}
			return nil
		},
	}
	show := &Command{
		Name: "show",
		Args: []string{"<id>"},
		Desc: "Show a profile as YAML.",
		Run: func(c *Ctx) error {
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			p, err := core.FindProfile(f, c.Args[0])
			if err != nil {
				return err
			}
			var b bytes.Buffer
			enc := yaml.NewEncoder(&b)
			enc.SetIndent(2)
			if err := enc.Encode(p); err != nil {
				return err
			}
			c.Println(strings.TrimRight(b.String(), "\n"))
			return nil
		},
	}
	remove := &Command{
		Name:    "remove",
		Args:    []string{"<id>"},
		Desc:    "Remove a profile from Hilvan. Its CLI config dir and login are left untouched.",
		Options: []*Option{{Flags: "-y, --yes", Desc: "do not ask for confirmation"}},
		Run: func(c *Ctx) error {
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			p, err := core.FindProfile(f, c.Args[0])
			if err != nil {
				return err
			}
			if !c.Bool("yes") && c.App.Interactive && !c.App.Confirm("Remove profile "+p.ID+"? ("+dirLabel(p)+" is kept)", true) {
				return nil
			}
			label := dirLabel(p)
			if f, err = core.RemoveProfile(f, p.ID); err != nil {
				return err
			}
			if err := c.saveProfiles(f); err != nil {
				return err
			}
			c.Println("✔ profile " + c.Args[0] + " removed; " + label + " was not deleted")
			return nil
		},
	}
	login := &Command{
		Name: "login",
		Args: []string{"<id>"},
		Desc: "Open the official CLI login for this profile's isolated config dir.",
		Run: func(c *Ctx) error {
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			p, err := core.FindProfile(f, c.Args[0])
			if err != nil {
				return err
			}
			if p.Destination.Launcher == domain.LauncherNative && p.ConfigDir != nil {
				if err := core.EnsureConfigDir(*p.ConfigDir); err != nil {
					return err
				}
			}
			adapter, _ := adapters.For(p.App)
			inv, err := launchers.BuildInvocation(p.Destination, adapter.LoginCommand(p, c.App.Host.Env), c.App.Host)
			if err != nil {
				return err
			}
			c.Println("→ " + p.App + " login for " + p.ID + " (" + dirLabel(p) + ")")
			code, err := process.RunInteractive(inv.Command, inv.Args, inv.Env, "")
			if err != nil {
				return err
			}
			if code != 0 {
				return fmt.Errorf("login exited with %d", code)
			}
			c.Println(formatCheck(core.CheckProfile(p, c.checkDeps())))
			return nil
		},
	}
	check := &Command{
		Name:    "check",
		Args:    []string{"[id]"},
		Desc:    "Verify executable, config dir and active account of one or all profiles.",
		Options: []*Option{{Flags: "--json", Desc: "print as JSON"}},
		Run: func(c *Ctx) error {
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			targets := f.Profiles
			if len(c.Args) > 0 {
				p, err := core.FindProfile(f, c.Args[0])
				if err != nil {
					return err
				}
				targets = []domain.Profile{*p}
			}
			if len(targets) == 0 {
				c.Println("No profiles yet. Add one with: hilvan profile add")
				return nil
			}
			var checks []core.ProfileCheck
			for i := range targets {
				checks = append(checks, core.CheckProfile(&targets[i], c.checkDeps()))
			}
			shared := core.SharedAccountWarnings(checks)
			if c.Bool("json") {
				if err := printJSON(c, struct {
					Checks   []core.ProfileCheck `json:"checks"`
					Warnings []string            `json:"warnings"`
				}{checks, shared}); err != nil {
					return err
				}
			} else {
				for _, ch := range checks {
					c.Println(formatCheck(ch))
				}
				for _, w := range shared {
					c.Println("! " + w)
				}
			}
			for _, ch := range checks {
				if len(ch.Issues) > 0 {
					c.Fail()
				}
			}
			return nil
		},
	}
	return (&Command{Name: "profile", Desc: "Manage agent profiles (one per account and environment)."}).Add(add, set, tools, list, show, remove, login, check)
}
