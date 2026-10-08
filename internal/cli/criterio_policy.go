package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/store"
)

func fmtDecision(value string, p float64, source string, why []criterio.Reason, label, disp string) string {
	head := fmt.Sprintf("%s: %s  (p=%s, %s", label, value, fixed(p, 2), source)
	if disp != "" {
		head += ", " + disp
	}
	lines := []string{head + ")"}
	for i, r := range why {
		if i == 4 {
			break
		}
		sign := ""
		if r.Weight >= 0 {
			sign = "+"
		}
		lines = append(lines, "    · "+r.Signal+" ("+sign+fixed(r.Weight, 2)+")")
	}
	return strings.Join(lines, "\n")
}

func criterioCommand() *Command {
	ask := &Command{
		Name: "ask",
		Args: []string{"<title>"},
		Desc: "Ask Criterio about a task brief: type, complexity and data sensitivity.",
		Options: []*Option{
			{Flags: "--text <text>", Desc: "brief body"},
			{Flags: "--criteria <n>", Desc: "number of acceptance criteria"},
			{Flags: "--classification <c>", Desc: "current classification", Default: "personal"},
			{Flags: "--json", Desc: "print as JSON"},
		},
		Run: func(c *Ctx) error {
			engine, warning, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			if warning != "" {
				c.Eprintln("! " + warning)
			}
			s := criterio.TaskSignals{Title: c.Args[0], Text: c.Str("text")}
			if c.Has("criteria") {
				n, _ := strconv.Atoi(c.Str("criteria"))
				s.CriteriaCount = &n
			}
			typ := engine.TaskType(s)
			comp := engine.Complexity(s, typ.Value)
			cls := engine.SuggestClassification(s, c.Str("classification"), nil)
			typeDisp := engine.Dispose(typ.P, criterio.ActionLabel, 0)
			compDisp := engine.Dispose(comp.P, criterio.ActionLabel, 0)
			clsDisp := engine.Dispose(cls.P, criterio.ActionHardenClassification, 0)
			if c.Bool("json") {
				type decision struct {
					Value       any               `json:"value"`
					P           float64           `json:"p"`
					Why         []criterio.Reason `json:"why"`
					Source      string            `json:"source"`
					Disposition string            `json:"disposition"`
				}
				var clsValue any
				if cls.Value != "" {
					clsValue = cls.Value
				}
				return printJSON(c, struct {
					TaskType       decision `json:"taskType"`
					Complexity     decision `json:"complexity"`
					Classification decision `json:"classification"`
				}{
					decision{typ.Value, typ.P, typ.Why, typ.Source, typeDisp},
					decision{comp.Value, comp.P, comp.Why, comp.Source, compDisp},
					decision{clsValue, cls.P, cls.Why, cls.Source, clsDisp},
				})
			}
			c.Println(fmtDecision(typ.Value, typ.P, typ.Source, typ.Why, "type", typeDisp))
			c.Println(fmtDecision(num(comp.Value), comp.P, comp.Source, comp.Why, "complexity", compDisp))
			value := "keep " + c.Str("classification")
			if cls.Value != "" {
				value = "harden to " + cls.Value
			}
			c.Println(fmtDecision(value, cls.P, cls.Source, cls.Why, "classification", clsDisp))
			return nil
		},
	}
	teach := &Command{
		Name:    "teach",
		Args:    []string{"<title>"},
		Desc:    "Teach Criterio the right task type for a brief (corrections improve later suggestions).",
		Options: []*Option{{Flags: "--type <type>", Desc: strings.Join(domain.TaskTypes, " | "), Required: true}, {Flags: "--text <text>", Desc: "brief body"}},
		Run: func(c *Ctx) error {
			typ := c.Str("type")
			if !contains(domain.TaskTypes, typ) {
				return fmt.Errorf("--type must be one of: %s", strings.Join(domain.TaskTypes, ", "))
			}
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			engine.LearnTaskType(criterio.TaskSignals{Title: c.Args[0], Text: c.Str("text")}, typ)
			if err := config.SaveCriterio(c.App.criterioFile(), engine); err != nil {
				return err
			}
			c.Println(fmt.Sprintf("✔ learned: %q → %s", c.Args[0], typ))
			return nil
		},
	}
	stats := &Command{
		Name:    "stats",
		Desc:    "Show how much Criterio has learned and how well calibrated it is.",
		Options: []*Option{{Flags: "--json", Desc: "print as JSON"}},
		Run: func(c *Ctx) error {
			engine, _, err := c.App.loadCriterio()
			if err != nil {
				return err
			}
			s := engine.Stats()
			if c.Bool("json") {
				return printJSON(c, s)
			}
			c.Println(fmt.Sprintf("examples: type %d, sensitivity %d, acceptance %d", s.Examples["taskType"], s.Examples["sensitivity"], s.Examples["acceptance"]))
			brier := "n/a (no outcomes yet)"
			if s.Brier != nil {
				brier = fixed(*s.Brier, 3)
			}
			c.Println("brier score: " + brier + " (0 = perfect, 0.25 = coin flip)")
			for _, b := range s.Reliability {
				if b.Count > 0 {
					c.Println(fmt.Sprintf("  p %s–%s: predicted %s, observed %s (n=%d)", fixed(b.From, 1), fixed(b.To, 1), fixed(b.MeanP, 2), fixed(b.Observed, 2), b.Count))
				}
			}
			for _, p := range s.Profiles {
				c.Println(fmt.Sprintf("  %s: %s %% success (n=%s)", p.Key, fixed(p.Mean*100, 0), num(p.N)))
			}
			return nil
		},
	}
	reset := &Command{
		Name:    "reset",
		Desc:    "Forget everything Criterio learned (rules remain).",
		Options: []*Option{{Flags: "-y, --yes", Desc: "confirm"}},
		Run: func(c *Ctx) error {
			if !c.Bool("yes") {
				return errors.New("add --yes to delete the learned model")
			}
			if err := os.Remove(c.App.criterioFile()); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			c.Println("✔ Criterio model deleted; rules only from now on")
			return nil
		},
	}
	return (&Command{Name: "criterio", Desc: "Local decision engine: typed suggestions with confidence and reasons (no network)."}).Add(ask, teach, stats, reset)
}

func (c *Ctx) policyPaths() config.PolicyPaths {
	root := ""
	if s, err := store.Find(c.App.Cwd); err == nil {
		root = s.Root
	}
	return config.DefaultPolicyPaths(c.App.Platform, root)
}

// selectorJSON renders a selector like JSON.stringify (absent fields omitted).
func selectorJSON(s domain.Selector) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func policyCommand() *Command {
	show := &Command{
		Name:    "show",
		Desc:    "Show the effective policy and where each rule comes from.",
		Options: []*Option{{Flags: "--json", Desc: "print as JSON"}},
		Run: func(c *Ctx) error {
			paths := c.policyPaths()
			eff, err := config.LoadEffectivePolicy(paths)
			if err != nil {
				return err
			}
			if c.Bool("json") {
				type pathsJSON struct {
					Managed string `json:"managed"`
					User    string `json:"user"`
					Project string `json:"project,omitempty"`
				}
				rules := eff.Rules
				if rules == nil {
					rules = []domain.LayeredRule{}
				}
				return printJSON(c, struct {
					Paths           pathsJSON            `json:"paths"`
					Classifications []string             `json:"classifications"`
					Rules           []domain.LayeredRule `json:"rules"`
					Warnings        []string             `json:"warnings"`
				}{pathsJSON{paths.Managed, paths.User, paths.Project}, eff.Classifications, rules, eff.Warnings})
			}
			files := "files: managed " + paths.Managed + " · user " + paths.User
			if paths.Project != "" {
				files += " · project " + paths.Project
			}
			c.Println(files)
			c.Println("classifications: " + strings.Join(eff.Classifications, " < "))
			if len(eff.Rules) == 0 {
				c.Println("rules: none (only profile `accepts` and required tools apply)")
			}
			for _, r := range eff.Rules {
				enforced := ""
				if r.Enforced {
					enforced = ", ENFORCED"
				}
				c.Println(fmt.Sprintf("- [%s%s] %s: %s from %s to %s — %s", r.Layer, enforced, r.ID, r.Effect, selectorJSON(r.From), selectorJSON(r.To), r.Reason))
			}
			for _, w := range eff.Warnings {
				c.Println("! " + w)
			}
			return nil
		},
	}
	check := &Command{
		Name:    "check",
		Args:    []string{"<task>"},
		Desc:    "Explain whether a profile may receive a task, and why.",
		Options: []*Option{{Flags: "-p, --profile <id>", Desc: "profile to check", Required: true}},
		Run: func(c *Ctx) error {
			s, err := store.Find(c.App.Cwd)
			if err != nil {
				return err
			}
			t, err := c.resolveTask(s, c.Args[0])
			if err != nil {
				return err
			}
			profiles, err := config.LoadProfiles(c.App.profilesFile())
			if err != nil {
				return err
			}
			p, err := core.FindProfile(profiles, c.Str("profile"))
			if err != nil {
				return err
			}
			eff, err := config.LoadEffectivePolicy(config.DefaultPolicyPaths(c.App.Platform, s.Root))
			if err != nil {
				return err
			}
			ev := policy.Evaluate(eff, policy.TaskFacts{Classification: t.Classification, Requires: t.Requires}, p, nil)
			if ev.Eligible {
				extra := ""
				if ev.RequiresApproval {
					extra = " — approval required"
				}
				c.Println("✔ " + p.ID + " is eligible for " + t.Classification + " data" + extra)
			} else {
				c.Println("✖ " + p.ID + " is not eligible")
			}
			for _, x := range ev.Exclusions {
				enforced := ""
				if x.Enforced {
					enforced = ", enforced"
				}
				c.Println("  ✖ " + x.Reason + " [" + x.Source + enforced + "]")
			}
			for _, a := range ev.Approvals {
				c.Println("  ? " + a.Reason + " [rule " + a.RuleID + ", " + a.Layer + "]")
			}
			if !ev.Eligible {
				c.Fail()
			}
			return nil
		},
	}
	return (&Command{Name: "policy", Desc: "Inspect the layered data policy (managed → user → project)."}).Add(show, check)
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
