package cli

import (
	"fmt"
	"strings"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/jsfmt"
	"github.com/ferchosur/hilvan/internal/security"
	"github.com/ferchosur/hilvan/internal/store"
)

// DefaultClassifications are offered in prompts and help.
var DefaultClassifications = []string{"public", "personal", "corporate", "confidential"}

func short(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}

// clean is a single-line, length-bounded, sanitised string for the terminal (SR-001).
func clean(s string, max int) string {
	return security.SanitizeForTerminal(s, security.SanitizeOptions{SingleLine: true, MaxLength: max})
}

func sanitize(s string) string { return security.SanitizeForTerminal(s, security.SanitizeOptions{}) }

func pct(x float64) string { return jsfmt.Number(jsfmt.Round(x*100)) + " %" }

func num(x float64) string { return jsfmt.Number(x) }

func fixed(x float64, d int) string { return jsfmt.ToFixed(x, d) }

// padEnd pads to n characters (UTF-16 units, like String.padEnd).
func padEnd(s string, n int) string {
	if l := jsfmt.UTF16Len(s); l < n {
		return s + strings.Repeat(" ", n-l)
	}
	return s
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// FormatRun is one line describing a run.
func FormatRun(r domain.RunSummary) string {
	state := r.State
	if r.BlockedReason != "" {
		state += " (" + r.BlockedReason + ")"
	}
	parts := []string{state, "profile " + r.ProfileID}
	if u := r.Usage; u != nil {
		if u.CostUSD != nil {
			parts = append(parts, "~$"+fixed(*u.CostUSD, 4)+" estimated")
		}
		if u.InputTokens != nil {
			out := int64(0)
			if u.OutputTokens != nil {
				out = *u.OutputTokens
			}
			parts = append(parts, fmt.Sprintf("%d in / %d out tokens", *u.InputTokens, out))
		}
	}
	if len(r.Limits) > 0 {
		var ws []string
		for _, w := range r.Limits {
			ws = append(ws, w.Name+" "+pct(w.Utilization))
		}
		parts = append(parts, "limits "+strings.Join(ws, ", "))
	}
	if r.Diff != nil {
		parts = append(parts, fmt.Sprintf("%d files +%d −%d", r.Diff.FilesChanged, r.Diff.LinesAdded, r.Diff.LinesDeleted))
	}
	if r.Risk != nil {
		parts = append(parts, "risk "+num(*r.Risk)+"/10")
	}
	if r.MeetsCriteria != nil {
		parts = append(parts, "meets brief ~"+pct(*r.MeetsCriteria))
	}
	return strings.Join(parts, " · ")
}

func printEvent(c *Ctx, e adapters.Event) {
	switch e.Type {
	case adapters.EvSession:
		line := "▸ session " + clean(deref(e.SessionID), 60)
		if e.Model != "" {
			line += " (" + clean(e.Model, 40) + ")"
		}
		c.Println(line)
	case adapters.EvMessage:
		text := security.SanitizeForTerminal(e.Text, security.SanitizeOptions{MaxLength: 4000})
		lines := strings.Split(text, "\n")
		for i, l := range lines {
			lines[i] = "  │ " + l
		}
		c.Println(strings.Join(lines, "\n"))
	case adapters.EvTool:
		if e.Status != "completed" {
			line := "  · " + clean(e.Name, 40) + " " + e.Status
			if e.Summary != "" {
				line += ": " + clean(e.Summary, 120)
			}
			c.Println(line)
		}
	case adapters.EvRetry:
		c.Println(fmt.Sprintf("  ! retry %d: %s", e.Attempt, clean(e.Error, 80)))
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (a *App) profilesFile() string { return a.Platform.ProfilesFile() }

func (a *App) criterioFile() string { return config.CriterioFile(a.Platform.UserConfigDir()) }

func (a *App) loadCriterio() (*criterio.Criterio, string, error) {
	return config.LoadCriterio(a.criterioFile())
}

// openStore finds the project and recovers a run whose process died.
func (c *Ctx) openStore() (*store.ProjectStore, error) {
	s, err := store.Find(c.App.Cwd)
	if err != nil {
		return nil, err
	}
	if id, err := recoverStale(s); err != nil {
		return nil, err
	} else if id != "" {
		c.Eprintln("! a previous run of " + short(id) + " was interrupted; task marked uncertain")
	}
	return s, nil
}

func (c *Ctx) resolveTask(s *store.ProjectStore, ref string) (domain.Task, error) {
	id, err := s.ResolveTaskID(ref)
	if err != nil {
		return domain.Task{}, err
	}
	return s.LoadTask(id)
}
