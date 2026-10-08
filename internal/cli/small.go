package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/process"
	"github.com/ferchosur/hilvan/internal/security"
	"github.com/ferchosur/hilvan/internal/store"
	"github.com/ferchosur/hilvan/internal/version"
)

var recoverStale = core.RecoverStaleRun

func printJSON(c *Ctx, v any) error {
	data, err := store.MarshalIndent(v)
	if err != nil {
		return err
	}
	_, err = c.Out().Write(data)
	return err
}

// ── doctor ────────────────────────────────────────────────────────────────

type checkResult struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail | info
	Detail string `json:"detail"`
}

func checkTool(name string, args []string, required bool) checkResult {
	r, _ := process.Run(name, args, process.Options{})
	if r.Code == 0 && r.Err == "" {
		out := strings.TrimSpace(r.Stdout)
		if out == "" {
			out = strings.TrimSpace(r.Stderr)
		}
		line, _, _ := strings.Cut(out, "\n")
		return checkResult{name, "ok", strings.TrimSuffix(line, "\r")}
	}
	reason := fmt.Sprintf("exit %d", r.Code)
	if r.Err == "not_found" {
		reason = "not found in PATH"
	} else if r.Err != "" {
		reason = "exit " + r.Err
	}
	status := "warn"
	if required {
		status = "fail"
	}
	return checkResult{name, status, reason}
}

var microsoft = regexp.MustCompile(`(?i)microsoft`)

func runDoctor(a *App) []checkResult {
	results := []checkResult{
		{"hilvan", "ok", version.Version + " (" + runtime.GOOS + "/" + runtime.GOARCH + ")"},
		checkTool("git", []string{"--version"}, true),
		checkTool("claude", []string{"--version"}, false),
		checkTool("codex", []string{"--version"}, false),
	}
	switch a.Platform.OS {
	case "win32":
		if r, _ := process.Run("wsl.exe", []string{"--version"}, process.Options{}); r.Code == 0 && r.Err == "" {
			results = append(results, checkResult{"wsl", "ok", "available (wsl launcher)"})
		} else {
			results = append(results, checkResult{"wsl", "info", "not available; wsl launcher disabled"})
		}
	case "linux":
		if v, err := os.ReadFile("/proc/version"); err == nil && microsoft.Match(v) {
			results = append(results, checkResult{"wsl", "info", "running inside WSL (windows launcher)"})
		}
	}
	file := a.profilesFile()
	if perms := security.LooseConfigPermissions(file); perms != "" {
		return append(results, checkResult{"profiles", "fail", perms + "; run: chmod 600"})
	}
	f, err := config.LoadProfiles(file)
	switch {
	case err != nil:
		results = append(results, checkResult{"profiles", "fail", err.Error()})
	case len(f.Profiles) > 0:
		results = append(results, checkResult{"profiles", "ok", fmt.Sprintf("%d configured (%s)", len(f.Profiles), file)})
		// wsl profiles keep an absolute executable from profile creation; nvm upgrades move it.
		for _, p := range f.Profiles {
			if p.Destination.Launcher == "wsl" && p.Destination.Executable == "" {
				results = append(results, checkResult{"profile " + p.ID, "warn", "wsl profile without an executable path; run: hilvan profile set " + p.ID + " --rediscover"})
			}
		}
	default:
		results = append(results, checkResult{"profiles", "info", "none yet (" + file + ")"})
	}
	return results
}

var icons = map[string]string{"ok": "✔", "warn": "!", "fail": "✖", "info": "·"}

func doctorCommand() *Command {
	return &Command{
		Name:    "doctor",
		Desc:    "Check the environment: git, agent CLIs, WSL and profile configuration.",
		Options: []*Option{{Flags: "--json", Desc: "print results as JSON"}},
		Run: func(c *Ctx) error {
			results := runDoctor(c.App)
			if c.Bool("json") {
				if err := printJSON(c, results); err != nil {
					return err
				}
			} else {
				width := 0
				for _, r := range results {
					width = max(width, len(r.Name))
				}
				for _, r := range results {
					c.Println(icons[r.Status] + " " + padEnd(r.Name, width) + "  " + r.Detail)
				}
			}
			for _, r := range results {
				if r.Status == "fail" {
					c.Fail()
				}
			}
			return nil
		},
	}
}

// ── audit ─────────────────────────────────────────────────────────────────

func auditCommand() *Command {
	return (&Command{Name: "audit", Desc: "Inspect the project audit log."}).Add(&Command{
		Name: "verify",
		Args: []string{"[file]"},
		Desc: "Verify the hash chain of the audit log (default: .hilvan/events.jsonl).",
		Run: func(c *Ctx) error {
			target := filepath.Join(c.App.Cwd, ".hilvan", "events.jsonl")
			if len(c.Args) > 0 {
				target = c.Args[0]
			}
			r, err := store.VerifyAudit(target, 0)
			if err != nil {
				return err
			}
			if r.OK {
				c.Println(fmt.Sprintf("✔ audit log intact (%d events) — %s", r.Events, target))
			} else {
				c.Eprintln(fmt.Sprintf("✖ audit log broken at line %d: %s — %s", r.Broken, r.Reason, target))
				c.Fail()
			}
			return nil
		},
	})
}

// ── stats ─────────────────────────────────────────────────────────────────

func statsCommand() *Command {
	return &Command{
		Name:    "stats",
		Desc:    "Usage per profile in this project (as reported by the agent CLIs) and Criterio calibration.",
		Options: []*Option{{Flags: "--json", Desc: "print as JSON"}},
		Run: func(c *Ctx) error {
			s, err := store.Find(c.App.Cwd)
			if err != nil {
				return err
			}
			tasks, err := s.ListTasks()
			if err != nil {
				return err
			}
			usage := core.UsageByProfile(tasks)
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			crit := engine.Stats()
			if c.Bool("json") {
				return printJSON(c, map[string]any{"usage": usage, "criterio": crit})
			}
			if len(usage) == 0 {
				c.Println("No runs yet in this project.")
			}
			for _, u := range usage {
				var states []string
				for _, st := range u.StateOrder {
					states = append(states, fmt.Sprintf("%s %d", st, u.States[st]))
				}
				cost := "cost not reported"
				if u.CostRuns > 0 {
					cost = fmt.Sprintf("~$%s estimated (%d runs)", fixed(u.CostUSD, 4), u.CostRuns)
				}
				c.Println(fmt.Sprintf("%s: %d runs (%s) · %s · %d in / %d out tokens", u.ProfileID, u.Runs, strings.Join(states, ", "), cost, u.InputTokens, u.OutputTokens))
				if l := u.LastLimits; l != nil {
					var ws []string
					for _, w := range l.Windows {
						ws = append(ws, w.Name+" "+pct(w.Utilization))
					}
					at := l.At
					if len(at) > 16 {
						at = at[:16]
					}
					c.Println("   last reported limits (" + strings.Replace(at, "T", " ", 1) + "): " + strings.Join(ws, ", "))
				}
				if u.Accepted+u.Discarded > 0 {
					c.Println(fmt.Sprintf("   outcomes: %d accepted, %d discarded", u.Accepted, u.Discarded))
				}
				if u.Reviews > 0 {
					c.Println(fmt.Sprintf("   cross reviews performed: %d", u.Reviews))
				}
			}
			brier := "n/a"
			if crit.Brier != nil {
				brier = fixed(*crit.Brier, 3)
			}
			c.Println(fmt.Sprintf("criterio: %d type examples · brier %s", crit.Examples["taskType"], brier))
			return nil
		},
	}
}
