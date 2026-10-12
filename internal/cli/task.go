package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/gitops"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/security"
	"github.com/ferchosur/hilvan/internal/store"
)

func initCommand() *Command {
	return &Command{
		Name: "init",
		Desc: "Make the current git repository a Hilvan project (.hilvan/, ignored by git).",
		Options: []*Option{
			{Flags: "--base <branch>", Desc: "base branch for task worktrees (default: current branch)"},
			{Flags: "--setup <cmd>", Desc: `command run in each new worktree, e.g. "npm ci"`},
		},
		Run: func(c *Ctx) error {
			root, err := gitops.RepoRoot(c.App.Cwd, gitops.DefaultGit)
			if err != nil {
				return err
			}
			base := c.Str("base")
			if base == "" {
				if base, err = gitops.CurrentBranch(root, gitops.DefaultGit); err != nil {
					return err
				}
			}
			var hook []string
			if v := c.Str("setup"); v != "" {
				hook = strings.Split(v, " ")
			}
			s := store.NewProjectStore(root)
			if _, err := s.Init(filepath.Base(root), base, hook); err != nil {
				return err
			}
			if err := s.Audit(store.AuditEvent{Type: "project.initialised", Data: map[string]any{"baseBranch": base}}); err != nil {
				return err
			}
			c.Println("✔ Hilvan project initialised in " + root + "/.hilvan (base branch: " + base + ")")
			c.Println(`  next: hilvan task new "<title>" --objective "…" --criteria "…"`)
			return nil
		},
	}
}

func printTask(c *Ctx, t domain.Task) {
	c.Println(t.ID + "  [" + t.State + "]  " + clean(t.Title, 200))
	complexity := "—"
	if t.Complexity != nil {
		complexity = num(*t.Complexity)
	}
	line := "  type " + or(t.TaskType, "—") + " · complexity " + complexity + " · " + t.Classification + " · " + t.Mode
	if t.Branch != nil {
		line += " · branch " + *t.Branch
	}
	if t.ProfileID != nil {
		line += " · profile " + *t.ProfileID
	}
	c.Println(line)
	for _, r := range t.Runs {
		c.Println("  run " + short(r.ID) + ": " + FormatRun(r))
	}
}

func (c *Ctx) catalog() (criterio.ToolCatalog, []domain.Profile, error) {
	f, err := c.loadProfiles()
	if err != nil {
		return nil, nil, err
	}
	return criterio.BuildToolCatalog(f.Profiles), f.Profiles, nil
}

func taskNew() *Command {
	return &Command{
		Name: "new",
		Args: []string{"<title>"},
		Desc: "Create a task from a brief. Criterio suggests type, complexity and classification.",
		Options: []*Option{
			{Flags: "--objective <text>", Desc: "what should be achieved"},
			{Flags: "--criteria <text>", Desc: "acceptance criterion (repeatable)", Collect: true},
			{Flags: "--constraint <text>", Desc: "constraint (repeatable)", Collect: true},
			{Flags: "--classification <c>", Desc: strings.Join(DefaultClassifications, " | "), Default: "personal"},
			{Flags: "--mode <mode>", Desc: "write (own worktree) | read", Default: "write"},
			{Flags: "--requires <list>", Desc: "required tools/capabilities, e.g. jira"},
			{Flags: "--type <type>", Desc: strings.Join(domain.TaskTypes, " | ")},
			{Flags: "--brief-file <file>", Desc: "read the objective from a Markdown file"},
		},
		Run: func(c *Ctx) error {
			title, mode, typ := c.Args[0], c.Str("mode"), c.Str("type")
			if mode != "write" && mode != "read" {
				return errors.New("--mode must be write or read")
			}
			if typ != "" && !contains(domain.TaskTypes, typ) {
				return errors.New("--type must be one of " + strings.Join(domain.TaskTypes, ", "))
			}
			s, err := c.openStore()
			if err != nil {
				return err
			}
			objective := c.Str("objective")
			if f := c.Str("brief-file"); f != "" {
				data, err := os.ReadFile(f)
				if err != nil {
					return err
				}
				objective = string(data)
			}
			b := core.Brief{Title: title, Objective: objective, Criteria: c.All("criteria"), Constraints: c.All("constraint"),
				Classification: c.Str("classification"), Mode: mode, Requires: c.List("requires")}
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			// FR-021: which tools does this brief need? Routing depends on it, so the user confirms.
			catalog, _, err := c.catalog()
			if err != nil {
				return err
			}
			var text []string
			for _, p := range append(append([]string{objective}, b.Criteria...), b.Constraints...) {
				if p != "" {
					text = append(text, p)
				}
			}
			signals := criterio.TaskSignals{Title: title, Text: strings.Join(text, "\n")}
			toolsLearned, toolHint := false, ""
			if len(catalog) > 0 {
				t := engine.RequiredTools(signals, catalog)
				switch {
				case b.Requires != nil:
					engine.LearnRequiredTools(signals, catalog, b.Requires)
					toolsLearned = true
				case len(t.Value) > 0 && c.App.Interactive:
					var choices []Choice
					for _, x := range t.PerTool {
						hint := "p=" + fixed(x.P, 2)
						if len(x.Why) > 0 {
							hint += " · " + x.Why[0].Signal
						}
						choices = append(choices, Choice{Value: x.Tool, Label: x.Tool, Hint: hint})
					}
					if picked, ok := c.App.MultiSelect("Criterio thinks this task needs: "+strings.Join(t.Value, ", ")+". Confirm the tools it requires", choices, t.Value, false); ok {
						b.Requires = picked
						engine.LearnRequiredTools(signals, catalog, picked)
						toolsLearned = true
					}
				case len(t.Value) > 0:
					var why []string
					for i, w := range t.Why {
						if i == 2 {
							break
						}
						why = append(why, w.Signal)
					}
					toolHint = "  ! Criterio thinks this task needs " + strings.Join(t.Value, ", ") + " (" + strings.Join(why, "; ") + "). If so, re-create it with --requires " + strings.Join(t.Value, ",")
				}
			}
			sg := core.Suggest(engine, b)
			typeDisp := engine.Dispose(sg.TaskType.P, criterio.ActionLabel, 0)
			chosen := typ
			if chosen == "" && typeDisp == "auto" {
				chosen = sg.TaskType.Value
			}
			b.TaskType, b.Complexity = chosen, sg.Complexity.Value
			var learner *criterio.Criterio
			if typ != "" {
				learner = engine
			}
			created, err := core.CreateTask(s, b, learner)
			if err != nil {
				return err
			}
			if typ != "" || toolsLearned {
				if err := config.SaveCriterio(c.App.criterioFile(), engine); err != nil {
					return err
				}
			}
			c.Println("✔ task " + short(created.ID) + " created: " + clean(title, 200))
			how := typeDisp
			if typ != "" {
				how = "you chose " + typ
			}
			c.Println("  criterio: type " + sg.TaskType.Value + " (p=" + fixed(sg.TaskType.P, 2) + ", " + how + ") · complexity " + num(sg.Complexity.Value) + "/10")
			if v := sg.Classification.Value; v != "" {
				var why []string
				for i, r := range sg.Classification.Why {
					if i == 3 {
						break
					}
					why = append(why, r.Signal)
				}
				c.Println(`  ! Criterio suggests classification "` + v + `" (` + strings.Join(why, ", ") + "). Re-create with --classification " + v + " if it applies.")
			}
			if len(created.Requires) > 0 {
				c.Println("  requires: " + strings.Join(created.Requires, ", "))
			}
			if toolHint != "" {
				c.Println(toolHint)
			}
			c.Println("  next: hilvan task recommend " + short(created.ID) + "   (or: hilvan task run " + short(created.ID) + " --profile <id>)")
			return nil
		},
	}
}

func taskRun() *Command {
	return &Command{
		Name: "run",
		Args: []string{"<id>"},
		Desc: "Run a task with a profile; streams the agent output and records the run.",
		Options: []*Option{
			{Flags: "-p, --profile <id>", Desc: "profile that runs the task", Required: true},
			{Flags: "--timeout <minutes>", Desc: "cancel the run after this many minutes", Default: "60"},
			{Flags: "--idle-timeout <minutes>", Desc: "cancel the run after this many minutes without any output", Default: minutesDefault(adapters.DefaultInactivity)},
			{Flags: "--approve", Desc: "approve sending the brief when a policy rule requires approval (non-interactive)"},
		},
		Run: func(c *Ctx) error {
			limit, err := minutesOption(c, "timeout")
			if err != nil {
				return err
			}
			idle, err := minutesOption(c, "idle-timeout")
			if err != nil {
				return err
			}
			s, err := c.openStore()
			if err != nil {
				return err
			}
			taskID, err := s.ResolveTaskID(c.Args[0])
			if err != nil {
				return err
			}
			f, err := c.loadProfiles()
			if err != nil {
				return err
			}
			p, err := core.FindProfile(f, c.Str("profile"))
			if err != nil {
				return err
			}
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			if p.Routing == "reserved" {
				t, err := s.LoadTask(taskID)
				if err != nil {
					return err
				}
				needed := false
				for _, r := range t.Requires {
					needed = needed || contains(p.Tools.Provides, r)
				}
				if !needed {
					c.Eprintln("! " + p.ID + " is reserved for tasks that need its tools (" + or(strings.Join(p.Tools.Provides, ", "), "none declared") + "); this task does not require them.")
				}
			}
			c.Println("→ running " + short(taskID) + " with " + p.ID + " (" + p.App + " @ " + p.Destination.Launcher + ")")
			eff, err := config.LoadEffectivePolicy(config.DefaultPolicyPaths(c.App.Platform, s.Root))
			if err != nil {
				return err
			}
			for _, w := range eff.Warnings {
				c.Eprintln("! policy: " + w)
			}
			// Ctrl+C asks the agent to stop; the runner escalates if it does not.
			sigs := make(chan os.Signal, 1)
			var handle *adapters.RunHandle
			signal.Notify(sigs, os.Interrupt)
			defer signal.Stop(sigs)
			go func() {
				for range sigs {
					c.Eprintln("\n! cancelling… (Ctrl+C again to force)")
					if handle != nil {
						handle.Cancel("")
					}
					signal.Stop(sigs)
				}
			}()
			report, err := core.RunTask(s, taskID, p, core.RunDeps{
				ProfileLocks: filepath.Join(c.App.Platform.UserConfigDir(), "locks"),
				Host:         c.App.Host,
				Policy:       &eff,
				Approve: func(ev policy.Evaluation, brief string) bool {
					c.Println("\n? " + p.ID + " needs your approval to receive this task:")
					for _, a := range ev.Approvals {
						c.Println("  · " + clean(a.Reason, 200) + " [rule " + a.RuleID + ", " + a.Layer + "]")
					}
					c.Println("──── exactly what will be sent (brief) ────")
					c.Println(sanitize(brief))
					c.Println("───────────────────────────────────────────")
					if c.Bool("approve") {
						return true
					}
					return c.App.Interactive && c.App.Confirm("Send it to "+p.ID+"?", false)
				},
				Criterio:   engine,
				RunOptions: adapters.RunOptions{Timeout: limit, Inactivity: idle},
				OnEvent:    func(e adapters.Event) { printEvent(c, e) },
				OnStart:    func(h *adapters.RunHandle) { handle = h },
			})
			if err != nil {
				return err
			}
			icon := "!"
			if report.Run.State == domain.RunCompleted {
				icon = "✔"
			}
			c.Println("\n" + icon + " " + FormatRun(report.Run))
			if report.Run.Error != "" {
				c.Println("  error: " + clean(report.Run.Error, 300))
			}
			if r := report.Run.Risk; r != nil && engine.Dispose(0.8, criterio.ActionSuggestReview, *r) == "escalate" {
				c.Println("  ! high-risk diff: ask another profile to review it before accepting (cross review, M5).")
			}
			next := "inspect with: hilvan task log " + short(taskID)
			switch report.Task.State {
			case domain.TaskInReview:
				next = "review the changes on branch " + or(deref(report.Task.Branch), "(read-only task)")
			case domain.TaskBlocked:
				next = "decide: wait for the limit to reset, or hand off to another profile (M4)"
			}
			c.Println("  task is now " + report.Task.State + "; next: " + next)
			if report.Run.State != domain.RunCompleted {
				c.Fail()
			}
			return nil
		},
	}
}

func taskRecommend() *Command {
	return &Command{
		Name:    "recommend",
		Args:    []string{"<id>"},
		Desc:    "Which profiles may run this task, which are excluded and why, and Criterio's ranking.",
		Options: []*Option{{Flags: "--json", Desc: "print as JSON"}},
		Run: func(c *Ctx) error {
			s, err := c.openStore()
			if err != nil {
				return err
			}
			t, err := c.resolveTask(s, c.Args[0])
			if err != nil {
				return err
			}
			catalog, profiles, err := c.catalog()
			if err != nil {
				return err
			}
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			eff, err := config.LoadEffectivePolicy(config.DefaultPolicyPaths(c.App.Platform, s.Root))
			if err != nil {
				return err
			}
			history, err := s.ListTasks()
			if err != nil {
				return err
			}
			rec := core.Recommend(eff, t, profiles, engine, history, uint32(time.Now().UnixMilli()))
			brief, err := s.Brief(t.ID)
			if err != nil {
				return err
			}
			var inferred []string
			for _, x := range engine.RequiredTools(criterio.TaskSignals{Title: t.Title, Text: brief}, catalog).Value {
				if !contains(t.Requires, x) {
					inferred = append(inferred, x)
				}
			}
			if c.Bool("json") {
				type ranked struct {
					ID               string            `json:"id"`
					Score            float64           `json:"score"`
					Why              []criterio.Reason `json:"why"`
					RequiresApproval bool              `json:"requiresApproval"`
				}
				type exclusion struct {
					Reason   string `json:"reason"`
					Source   string `json:"source"`
					Enforced bool   `json:"enforced"`
				}
				type excluded struct {
					ID         string      `json:"id"`
					Exclusions []exclusion `json:"exclusions"`
				}
				out := struct {
					TaskType string     `json:"taskType"`
					Ranked   []ranked   `json:"ranked"`
					Excluded []excluded `json:"excluded"`
				}{TaskType: rec.TaskType, Ranked: []ranked{}, Excluded: []excluded{}}
				for _, r := range rec.Ranked {
					out.Ranked = append(out.Ranked, ranked{r.Profile.ID, r.Score, r.Why, r.Evaluation.RequiresApproval})
				}
				for _, x := range rec.Excluded {
					ex := excluded{ID: x.Profile.ID, Exclusions: []exclusion{}}
					for _, e := range x.Evaluation.Exclusions {
						ex.Exclusions = append(ex.Exclusions, exclusion{e.Reason, e.Source, e.Enforced})
					}
					out.Excluded = append(out.Excluded, ex)
				}
				return printJSON(c, out)
			}
			head := "task " + short(t.ID) + " · " + t.Classification + " · type " + rec.TaskType
			if len(t.Requires) > 0 {
				head += " · requires " + strings.Join(t.Requires, ", ")
			}
			c.Println(head)
			if len(rec.Ranked) == 0 {
				c.Println("✖ no eligible profile")
			}
			for i, r := range rec.Ranked {
				star := " "
				if i == 0 {
					star = "★"
				}
				approval := ""
				if r.Evaluation.RequiresApproval {
					approval = " · approval required"
				}
				c.Println(star + " " + r.Profile.ID + " (" + r.Profile.App + " @ " + core.DescribeDestination(r.Profile) + ") score " + fixed(r.Score, 2) + approval)
				for _, w := range r.Why {
					c.Println("     · " + w.Signal)
				}
			}
			for _, x := range rec.Excluded {
				var reasons []string
				for _, e := range x.Evaluation.Exclusions {
					r := e.Reason
					if e.Enforced {
						r += " (enforced)"
					}
					reasons = append(reasons, r)
				}
				c.Println("  ✖ " + x.Profile.ID + ": " + strings.Join(reasons, "; "))
			}
			if len(inferred) > 0 {
				c.Println("  ! Criterio thinks this task also needs " + strings.Join(inferred, ", ") + "; if so: hilvan task requires " + short(t.ID) + " " + strings.Join(append(append([]string{}, t.Requires...), inferred...), ","))
			}
			if len(rec.Ranked) > 0 {
				c.Println("  next: hilvan task run " + short(t.ID) + " --profile " + rec.Ranked[0].Profile.ID + "  (you decide; nothing runs automatically)")
			}
			return nil
		},
	}
}

func taskCommand() *Command {
	list := &Command{
		Name:    "list",
		Desc:    "List tasks of this project.",
		Options: []*Option{{Flags: "--json", Desc: "print as JSON"}},
		Run: func(c *Ctx) error {
			s, err := c.openStore()
			if err != nil {
				return err
			}
			tasks, err := s.ListTasks()
			if err != nil {
				return err
			}
			if c.Bool("json") {
				if tasks == nil {
					tasks = []domain.Task{}
				}
				return printJSON(c, tasks)
			}
			if len(tasks) == 0 {
				c.Println(`No tasks yet. Create one with: hilvan task new "<title>"`)
				return nil
			}
			for _, t := range tasks {
				c.Println(fmt.Sprintf("%s  %s %s %s %s %s", short(t.ID), padEnd(t.State, 11), padEnd(or(t.TaskType, "—"), 9), padEnd(t.Classification, 12), padEnd(or(deref(t.ProfileID), "—"), 16), clean(t.Title, 60)))
			}
			return nil
		},
	}
	show := &Command{
		Name: "show",
		Args: []string{"<id>"},
		Desc: "Show a task, its brief and its runs.",
		Run: func(c *Ctx) error {
			s, err := c.openStore()
			if err != nil {
				return err
			}
			t, err := c.resolveTask(s, c.Args[0])
			if err != nil {
				return err
			}
			printTask(c, t)
			brief, err := s.Brief(t.ID)
			if err != nil {
				return err
			}
			c.Println("\n" + sanitize(brief))
			return nil
		},
	}
	logCmd := &Command{
		Name:    "log",
		Args:    []string{"<id>"},
		Desc:    "Print the stored agent events of the last run (secrets redacted).",
		Options: []*Option{{Flags: "--run <runId>", Desc: "a specific run"}},
		Run: func(c *Ctx) error {
			s, err := c.openStore()
			if err != nil {
				return err
			}
			t, err := c.resolveTask(s, c.Args[0])
			if err != nil {
				return err
			}
			runID := c.Str("run")
			if runID == "" && len(t.Runs) > 0 {
				runID = t.Runs[len(t.Runs)-1].ID
			}
			if runID == "" {
				c.Println("No runs yet.")
				return nil
			}
			file, err := s.RunFile(t.ID, runID)
			if err != nil {
				return err
			}
			text, _, _ := security.ReadTextSafe(file, 512*1024*1024)
			fmt.Fprint(c.Out(), sanitize(text))
			return nil
		},
	}
	requires := &Command{
		Name: "requires",
		Args: []string{"<id>", "<tools>"},
		Desc: `Set the tools a task requires (comma list, or "none"); Criterio learns from it.`,
		Run: func(c *Ctx) error {
			s, err := c.openStore()
			if err != nil {
				return err
			}
			t, err := c.resolveTask(s, c.Args[0])
			if err != nil {
				return err
			}
			if len(t.Runs) > 0 {
				return errors.New("the task already ran; create a new task or hand it off")
			}
			req := []string{}
			if c.Args[1] != "none" {
				req = SplitList(c.Args[1])
			}
			t.Requires = req
			if t, err = s.SaveTask(t, nil); err != nil {
				return err
			}
			anyReq := make([]any, len(req))
			for i, r := range req {
				anyReq[i] = r
			}
			if err := s.Audit(store.AuditEvent{Type: "task.requires_changed", TaskID: t.ID, Data: map[string]any{"requires": anyReq}}); err != nil {
				return err
			}
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			catalog, _, err := c.catalog()
			if err != nil {
				return err
			}
			if len(catalog) > 0 {
				brief, err := s.Brief(t.ID)
				if err != nil {
					return err
				}
				engine.LearnRequiredTools(criterio.TaskSignals{Title: t.Title, Text: brief}, catalog, req)
				if err := config.SaveCriterio(c.App.criterioFile(), engine); err != nil {
					return err
				}
			}
			c.Println("✔ task " + short(t.ID) + " requires: " + or(strings.Join(req, ", "), "nothing"))
			c.Println("  next: hilvan task recommend " + short(t.ID))
			return nil
		},
	}
	cancel := &Command{
		Name: "cancel",
		Args: []string{"<id>"},
		Desc: "Ask the running agent of a task to stop.",
		Run: func(c *Ctx) error {
			s, err := c.openStore()
			if err != nil {
				return err
			}
			id, err := s.ResolveTaskID(c.Args[0])
			if err != nil {
				return err
			}
			ok, err := core.CancelTask(s, id)
			if err != nil {
				return err
			}
			if ok {
				c.Println("✔ cancellation requested")
			} else {
				c.Println("No active run for that task.")
			}
			return nil
		},
	}
	discard := &Command{
		Name:    "discard",
		Args:    []string{"<id>"},
		Desc:    "Drop the task, its worktree and its branch without merging.",
		Options: []*Option{{Flags: "-y, --yes", Desc: "confirm"}},
		Run: func(c *Ctx) error {
			if !c.Bool("yes") {
				return errors.New("add --yes to discard the task and delete its branch")
			}
			s, err := c.openStore()
			if err != nil {
				return err
			}
			id, err := s.ResolveTaskID(c.Args[0])
			if err != nil {
				return err
			}
			t, err := discardWithLearning(c, s, id)
			if err != nil {
				return err
			}
			c.Println("✔ task " + short(t.ID) + " discarded")
			return nil
		},
	}
	return (&Command{Name: "task", Desc: "Create, run and inspect tasks."}).Add(taskNew(), list, show, logCmd, taskRun(), taskRecommend(), requires, cancel, discard)
}

func discardWithLearning(c *Ctx, s *store.ProjectStore, id string) (domain.Task, error) {
	engine, _, err := c.App.loadCriterio()
	if err != nil {
		return domain.Task{}, err
	}
	t, err := core.DiscardTask(s, id, nil, func(t domain.Task) { core.LearnOutcome(t, false, engine) })
	if err != nil {
		return t, err
	}
	return t, config.SaveCriterio(c.App.criterioFile(), engine)
}

// minutesOption reads a positive number of minutes from an option such as --timeout.
func minutesOption(c *Ctx, long string) (time.Duration, error) {
	m, err := strconv.ParseFloat(c.Str(long), 64)
	if err != nil || m <= 0 {
		return 0, fmt.Errorf("--%s must be a number of minutes greater than zero", long)
	}
	return time.Duration(m * float64(time.Minute)), nil
}

// minutesDefault formats a duration as the default text of a --...-timeout option.
func minutesDefault(d time.Duration) string { return strconv.FormatFloat(d.Minutes(), 'f', -1, 64) }
