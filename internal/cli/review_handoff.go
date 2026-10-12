package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/process"
	"github.com/ferchosur/hilvan/internal/store"
)

// editInEditor opens $VISUAL/$EDITOR on a private temp file (no shell; EDITOR may carry args).
func editInEditor(c *Ctx, stateDir, text string) (string, error) {
	file := filepath.Join(stateDir, fmt.Sprintf("handoff-edit-%d.md", os.Getpid()))
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		return "", err
	}
	defer os.Remove(file)
	editor := strings.TrimSpace(or(c.App.Host.Env["VISUAL"], c.App.Host.Env["EDITOR"]))
	if editor == "" {
		editor = "vi"
		if c.App.Platform.OS == "win32" {
			editor = "notepad"
		}
	}
	parts := strings.Fields(editor)
	code, err := process.RunInteractive(parts[0], append(parts[1:], file), nil, "")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("editor exited with %d", code)
	}
	data, err := os.ReadFile(file)
	return string(data), err
}

func handoffCommand() *Command {
	return &Command{
		Name: "handoff",
		Args: []string{"<task>"},
		Desc: "Hand a task over to another profile with a reviewable continuity package (you approve exactly what is sent).",
		Options: []*Option{
			{Flags: "--to <profile>", Desc: "profile that continues the task", Required: true},
			{Flags: "--edit", Desc: "edit the package in $EDITOR before approving"},
			{Flags: "--approve", Desc: "approve without prompting (non-interactive)"},
			{Flags: "--print", Desc: "only print the draft package; send nothing"},
		},
		Run: func(c *Ctx) error {
			s, err := store.Find(c.App.Cwd)
			if err != nil {
				return err
			}
			taskID, err := s.ResolveTaskID(c.Args[0])
			if err != nil {
				return err
			}
			profiles, err := c.loadProfiles()
			if err != nil {
				return err
			}
			to, err := core.FindProfile(profiles, c.Str("to"))
			if err != nil {
				return err
			}
			t, err := s.LoadTask(taskID)
			if err != nil {
				return err
			}
			var from *domain.Profile
			if t.ProfileID != nil {
				from, _ = profiles.Find(*t.ProfileID)
			}
			eff, err := config.LoadEffectivePolicy(config.DefaultPolicyPaths(c.App.Platform, s.Root))
			if err != nil {
				return err
			}
			draft, err := core.DraftHandoff(s, taskID, to, from, eff, nil)
			if err != nil {
				return err
			}
			if !draft.Evaluation.Eligible {
				for _, x := range draft.Evaluation.Exclusions {
					enforced := ""
					if x.Enforced {
						enforced = " (enforced)"
					}
					c.Eprintln("  ✖ " + x.Reason + enforced)
				}
				return errors.New(to.ID + " is not eligible to receive this task; nothing was sent")
			}
			if len(draft.Secrets) > 0 {
				var kinds []string
				for _, m := range draft.Secrets {
					if !contains(kinds, m.Kind) {
						kinds = append(kinds, m.Kind)
					}
				}
				c.Println(fmt.Sprintf("! %d possible secret(s) were redacted: %s", len(draft.Secrets), strings.Join(kinds, ", ")))
			}
			for _, a := range draft.Evaluation.Approvals {
				c.Println("? policy: " + a.Reason + " [rule " + a.RuleID + ", " + a.Layer + "]")
			}
			body := draft.Body
			if c.Bool("edit") {
				if body, err = editInEditor(c, s.Dir, body); err != nil {
					return err
				}
			}
			c.Println("──── continuity package for " + to.ID + " (sha256 " + core.SHA256(body)[:12] + ") ────")
			c.Println(sanitize(body))
			c.Println("────────────────────────────────────────────────────────")
			if c.Bool("print") {
				return nil
			}
			ok := c.Bool("approve")
			if !ok && c.App.Interactive {
				ok = c.App.Confirm("Send exactly this package to "+to.ID+"?", false)
			}
			if !ok {
				if err := core.CancelHandoff(s, draft); err != nil {
					return err
				}
				c.Println("✖ cancelled; nothing was sent")
				c.Fail()
				return nil
			}
			_, hash, _, err := core.ApproveHandoff(s, draft, body)
			if err != nil {
				return err
			}
			c.Println("✔ hand-off " + short(draft.ID) + " approved (sha256 " + hash[:12] + "); task now belongs to " + to.ID)
			c.Println("  next: hilvan task run " + short(taskID) + " --profile " + to.ID)
			return nil
		},
	}
}

func reviewCommand() *Command {
	return &Command{
		Name: "review",
		Args: []string{"<task>"},
		Desc: "Show the task diff with Criterio risk; with --by, ask another profile for a read-only review.",
		Options: []*Option{
			{Flags: "--by <profile>", Desc: "cross review with this profile (read-only)"},
			{Flags: "--stat", Desc: "only the list of changed files"},
			{Flags: "--approve", Desc: "approve sending the task to the reviewer if a policy rule requires it"},
			{Flags: "--timeout <minutes>", Desc: "cancel the cross review after this many minutes", Default: minutesDefault(adapters.DefaultReviewTimeout)},
			{Flags: "--idle-timeout <minutes>", Desc: "cancel the cross review after this many minutes without any output", Default: minutesDefault(adapters.DefaultInactivity)},
		},
		Run: func(c *Ctx) error {
			s, err := store.Find(c.App.Cwd)
			if err != nil {
				return err
			}
			taskID, err := s.ResolveTaskID(c.Args[0])
			if err != nil {
				return err
			}
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			if by := c.Str("by"); by != "" {
				f, err := c.loadProfiles()
				if err != nil {
					return err
				}
				reviewer, err := core.FindProfile(f, by)
				if err != nil {
					return err
				}
				c.Println("→ cross review of " + short(taskID) + " by " + reviewer.ID + " (read-only)")
				eff, err := config.LoadEffectivePolicy(config.DefaultPolicyPaths(c.App.Platform, s.Root))
				if err != nil {
					return err
				}
				limit, err := minutesOption(c, "timeout")
				if err != nil {
					return err
				}
				idle, err := minutesOption(c, "idle-timeout")
				if err != nil {
					return err
				}
				r, err := core.CrossReview(s, taskID, reviewer, core.RunDeps{
					Host:       c.App.Host,
					Policy:     &eff,
					RunOptions: adapters.RunOptions{Timeout: limit, Inactivity: idle},
					Approve: func(ev policy.Evaluation, brief string) bool {
						for _, a := range ev.Approvals {
							c.Println("? " + a.Reason + " [rule " + a.RuleID + "]")
						}
						c.Println(sanitize(brief))
						if c.Bool("approve") {
							return true
						}
						return c.App.Interactive && c.App.Confirm("Send it to "+reviewer.ID+"?", false)
					},
				})
				if err != nil {
					return err
				}
				c.Println(sanitize(r.Text))
				icon := "?"
				switch r.Verdict {
				case "approve":
					icon = "✔"
				case "changes_requested":
					icon = "✖"
				}
				c.Println("\n" + icon + " verdict: " + r.Verdict + " — saved in " + r.File)
				return nil
			}
			t, diff, truncated, err := core.ReviewDiff(s, taskID, nil)
			if err != nil {
				return err
			}
			var last *domain.RunSummary
			if len(t.Runs) > 0 {
				last = &t.Runs[len(t.Runs)-1]
			}
			c.Println(t.ID + "  [" + t.State + "]  " + clean(t.Title, 0))
			if t.Branch == nil {
				c.Println("read-only task: no diff. Last agent summary (unverified):")
				summary := "(none)"
				if last != nil && last.Summary != "" {
					summary = last.Summary
				}
				c.Println(sanitize(summary))
			} else {
				if last != nil && last.Diff != nil {
					d := last.Diff
					risk := engine.DiffRisk(criterio.DiffSignals{FilesChanged: d.FilesChanged, LinesAdded: d.LinesAdded, LinesDeleted: d.LinesDeleted, Paths: d.Paths})
					var why []string
					for _, w := range risk.Why {
						why = append(why, w.Signal)
					}
					c.Println("risk " + num(risk.Value) + "/10 · " + strings.Join(why, " · "))
					for _, p := range d.Paths {
						mark := "·"
						if criterio.IsSensitivePath(p) {
							mark = "⚠"
						}
						c.Println("  " + mark + " " + clean(p, 0))
					}
					if engine.Dispose(risk.P, criterio.ActionSuggestReview, risk.Value) == "escalate" && len(t.Reviews) == 0 {
						c.Println("  ! high risk: consider `hilvan review " + short(t.ID) + " --by <other-profile>` before accepting")
					}
				}
				if !c.Bool("stat") {
					more := ""
					if truncated {
						more = "\n… (diff truncated)"
					}
					c.Println("\n" + sanitize(diff) + more)
				}
			}
			for _, r := range t.Reviews {
				c.Println("review by " + r.ProfileID + ": " + r.Verdict)
			}
			if t.State == domain.TaskInReview {
				c.Println("\nnext: hilvan accept " + short(t.ID) + "   or   hilvan task discard " + short(t.ID) + " --yes")
			}
			return nil
		},
	}
}

func acceptCommand() *Command {
	return &Command{
		Name: "accept",
		Args: []string{"<task>"},
		Desc: "Accept a reviewed task: merge its branch into the base branch and clean up (Criterio learns).",
		Run: func(c *Ctx) error {
			s, err := store.Find(c.App.Cwd)
			if err != nil {
				return err
			}
			id, err := s.ResolveTaskID(c.Args[0])
			if err != nil {
				return err
			}
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			t, err := core.AcceptTask(s, id, engine, nil)
			if err != nil {
				return err
			}
			if err := config.SaveCriterio(c.App.criterioFile(), engine); err != nil {
				return err
			}
			merged := ""
			if t.Mode == "write" {
				merged = " and merged into " + deref(t.BaseBranch)
			}
			c.Println("✔ task " + short(t.ID) + " accepted" + merged)
			return nil
		},
	}
}
