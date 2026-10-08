package criterio

import (
	"math"
	"strings"

	"hilvan/internal/domain"
)

// BuiltinToolAliases are seed words (es/en) that reveal a brief needs a common enterprise tool
// (FR-021). Profiles add their own aliases, and Criterio learns from corrections.
var BuiltinToolAliases = map[string][]string{
	"jira":       {"jira", "ticket", "tickets", "incidencia", "incidencias", "issue", "issues", "sprint", "backlog", "epic", "historia de usuario", "user story"},
	"confluence": {"confluence", "wiki", "pagina de confluence", "espacio de confluence"},
	"slack":      {"slack", "canal de slack", "slack channel"},
	"teams":      {"microsoft teams", "teams", "canal de teams"},
	"github":     {"github", "pull request", "github actions"},
	"gitlab":     {"gitlab", "merge request"},
	"linear":     {"linear"},
	"notion":     {"notion"},
	"sharepoint": {"sharepoint", "onedrive"},
	"outlook":    {"outlook", "correo", "correos", "email", "calendario", "calendar", "reunion", "meeting"},
	"salesforce": {"salesforce", "crm", "oportunidades de venta"},
	"servicenow": {"servicenow", "change request", "incidente de servicenow"},
	"sap":        {"sap"},
	"datadog":    {"datadog", "monitor", "monitoreo", "metricas de produccion"},
	"sentry":     {"sentry", "error tracking"},
	"figma":      {"figma", "mockup", "prototipo de diseno"},
	"bigquery":   {"bigquery"},
	"snowflake":  {"snowflake", "data warehouse"},
}

// genericToolWords say nothing about the tool.
var genericToolWords = setOf([]string{"mcp", "claude", "ai", "api", "server", "servidor", "tool", "tools", "interno", "internal", "corp"})

// ToolEntry is one tool and the words that reveal it.
type ToolEntry struct {
	Tool    string
	Aliases []string
}

// ToolCatalog lists the tools some profile provides, in first-seen order.
type ToolCatalog []ToolEntry

// BuildToolCatalog merges id words, built-in seeds and profile aliases for each provided tool.
func BuildToolCatalog(profiles []domain.Profile) ToolCatalog {
	var out ToolCatalog
	pos := map[string]int{}
	for _, p := range profiles {
		for _, id := range p.Tools.Provides {
			var words []string
			for _, w := range strings.Split(id, "-") {
				if len(w) > 1 && !genericToolWords[w] {
					words = append(words, w)
				}
			}
			i, ok := pos[id]
			if !ok {
				i = len(out)
				pos[id] = i
				out = append(out, ToolEntry{Tool: id})
			}
			merged := append(append(append(append([]string{}, out[i].Aliases...), words...), BuiltinToolAliases[id]...), p.Tools.Aliases[id]...)
			out[i].Aliases = unique(merged)
		}
	}
	return out
}

// aliasHits returns the aliases found in the text, matching stemmed word sequences.
func aliasHits(text string, aliases []string) []string {
	toks := Tokenize(text)
	var hits []string
	for _, alias := range aliases {
		a := Tokenize(alias)
		if len(a) == 0 {
			continue
		}
		for i := 0; i+len(a) <= len(toks); i++ {
			match := true
			for j, w := range a {
				if toks[i+j] != w {
					match = false
					break
				}
			}
			if match {
				hits = append(hits, alias)
				break
			}
		}
	}
	return hits
}

func toolRule(text string, aliases []string) (p float64, why []Reason) {
	hits := aliasHits(text, aliases)
	if len(hits) == 0 {
		return 0.08, []Reason{}
	}
	why = []Reason{}
	for _, h := range hits[:min(3, len(hits))] {
		why = append(why, Reason{Signal: `menciona "` + h + `"`, Weight: 1})
	}
	return math.Min(0.97, 0.75+0.1*float64(len(hits))), why
}
