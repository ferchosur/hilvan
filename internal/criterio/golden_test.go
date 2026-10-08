package criterio

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/domain"
)

// Golden cases from the TypeScript implementation (scripts/parity/gen-g1.ts): the Go engine must
// replay the same 2.5k operations and produce the same decisions, rankings, state and stats.

type jsDiff struct {
	FilesChanged int      `json:"filesChanged"`
	LinesAdded   int      `json:"linesAdded"`
	LinesDeleted int      `json:"linesDeleted"`
	Paths        []string `json:"paths"`
}

func (d *jsDiff) signals() *DiffSignals {
	if d == nil {
		return nil
	}
	return &DiffSignals{FilesChanged: d.FilesChanged, LinesAdded: d.LinesAdded, LinesDeleted: d.LinesDeleted, Paths: d.Paths}
}

type jsTask struct {
	Title         string  `json:"title"`
	Text          string  `json:"text"`
	CriteriaCount *int    `json:"criteriaCount"`
	Diff          *jsDiff `json:"diff"`
}

func (s jsTask) signals() TaskSignals {
	return TaskSignals{Title: s.Title, Text: s.Text, CriteriaCount: s.CriteriaCount, Diff: s.Diff.signals()}
}

type jsRun struct {
	State             string  `json:"state"`
	ResultText        string  `json:"resultText"`
	PermissionDenials int     `json:"permissionDenials"`
	CriteriaCount     *int    `json:"criteriaCount"`
	Diff              *jsDiff `json:"diff"`
}

func (r jsRun) signals() RunSignals {
	return RunSignals{State: r.State, ResultText: r.ResultText, PermissionDenials: r.PermissionDenials, CriteriaCount: r.CriteriaCount, Diff: r.Diff.signals()}
}

type jsCandidate struct {
	ID            string   `json:"id"`
	App           string   `json:"app"`
	Preference    int      `json:"preference"`
	QuotaHeadroom *float64 `json:"quotaHeadroom"`
}

type golden struct {
	Profiles []struct {
		ID    string       `json:"id"`
		Tools domain.Tools `json:"tools"`
	} `json:"profiles"`
	Catalog map[string][]string `json:"catalog"`
	Ops     []struct {
		Op   string            `json:"op"`
		Args []json.RawMessage `json:"args"`
		Out  any               `json:"out"`
	} `json:"ops"`
	// StateText is JSON.stringify(engine.toJSON()) after the last operation, byte for byte.
	StateText string `json:"stateText"`
}

func loadGolden(t *testing.T) golden {
	f, err := os.Open("testdata/engine.golden.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		t.Fatal(err)
	}
	return g
}

func arg[T any](t *testing.T, raw json.RawMessage) T {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("arg %s: %v", raw, err)
	}
	return v
}

func generic(t *testing.T, v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// diff returns a description of the first difference, or "".
func diff(path string, got, want any) string {
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		if !ok {
			return fmt.Sprintf("%s: got %v (%T), want %v", path, got, got, w)
		}
		tol := 1e-9 * math.Max(1, math.Max(math.Abs(g), math.Abs(w)))
		if strings.Contains(path, ".weights.") {
			tol = 2e-5 // stored weights are rounded to 5 decimals
		}
		if math.Abs(g-w) > tol {
			return fmt.Sprintf("%s: got %v, want %v", path, g, w)
		}
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: got %T, want object", path, got)
		}
		keys := []string{}
		for k := range w {
			keys = append(keys, k)
		}
		for k := range g {
			if _, ok := w[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			if d := diff(path+"."+k, g[k], w[k]); d != "" {
				return d
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return fmt.Sprintf("%s: got %v, want %v", path, got, w)
		}
		for i := range w {
			if d := diff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
	default:
		if got != want {
			return fmt.Sprintf("%s: got %v, want %v", path, got, want)
		}
	}
	return ""
}

func TestGoldenEngine(t *testing.T) {
	g := loadGolden(t)
	profiles := make([]domain.Profile, len(g.Profiles))
	for i, p := range g.Profiles {
		profiles[i] = domain.Profile{ID: p.ID, Tools: p.Tools}
	}
	catalog := BuildToolCatalog(profiles)
	gotCatalog := map[string]any{}
	for _, e := range catalog {
		gotCatalog[e.Tool] = e.Aliases
	}
	if d := diff("catalog", generic(t, gotCatalog), generic(t, g.Catalog)); d != "" {
		t.Fatal(d)
	}

	c := New()
	failures := 0
	for i, op := range g.Ops {
		var out any
		a := op.Args
		switch op.Op {
		case "taskType":
			out = c.TaskType(arg[jsTask](t, a[0]).signals())
		case "complexity":
			out = c.Complexity(arg[jsTask](t, a[0]).signals(), arg[string](t, a[1]))
		case "diffRisk":
			out = c.DiffRisk(*arg[*jsDiff](t, a[0]).signals())
		case "sensitive":
			out = c.Sensitive(arg[jsTask](t, a[0]).signals())
		case "suggestClassification":
			out = c.SuggestClassification(arg[jsTask](t, a[0]).signals(), arg[string](t, a[1]), nil)
		case "meetsCriteria":
			out = c.MeetsCriteria(arg[jsRun](t, a[0]).signals())
		case "requiredTools":
			out = c.RequiredTools(arg[jsTask](t, a[0]).signals(), catalog)
		case "rankProfiles":
			cs := arg[[]jsCandidate](t, a[0])
			cands := make([]Candidate, len(cs))
			for j, x := range cs {
				cands[j] = Candidate{ID: x.ID, App: x.App, Preference: x.Preference, QuotaHeadroom: x.QuotaHeadroom}
			}
			out = c.RankProfiles(cands, arg[string](t, a[1]), uint32(arg[float64](t, a[2])))
		case "learnTaskType":
			c.LearnTaskType(arg[jsTask](t, a[0]).signals(), arg[string](t, a[1]))
		case "learnComplexity":
			c.LearnComplexity(arg[jsTask](t, a[0]).signals(), arg[string](t, a[1]), arg[float64](t, a[2]))
		case "learnSensitivity":
			c.LearnSensitivity(arg[jsTask](t, a[0]).signals(), arg[bool](t, a[1]))
		case "learnAcceptance":
			c.LearnAcceptance(arg[jsRun](t, a[0]).signals(), arg[bool](t, a[1]))
		case "recordOutcome":
			c.RecordOutcome(arg[string](t, a[0]), arg[string](t, a[1]), arg[bool](t, a[2]))
		case "learnRequiredTools":
			c.LearnRequiredTools(arg[jsTask](t, a[0]).signals(), catalog, arg[[]string](t, a[1]))
		case "state":
			out = c.State()
		case "stats":
			out = c.Stats()
		default:
			t.Fatalf("unknown op %s", op.Op)
		}
		if out == nil && op.Out == nil {
			continue
		}
		if d := diff(fmt.Sprintf("op %d %s", i, op.Op), generic(t, out), op.Out); d != "" {
			failures++
			if failures <= 5 {
				t.Error(d)
			}
		}
	}
	if failures > 0 {
		t.Fatalf("%d of %d operations differ", failures, len(g.Ops))
	}
	t.Logf("%d operations match the TypeScript engine", len(g.Ops))

	// model.json written by Go after the same history matches the TypeScript one. Weights are
	// byte-identical; probabilities can differ in the last bit (V8's Math.exp vs Go's math.Exp).
	got, err := json.Marshal(c.State())
	if err != nil {
		t.Fatal(err)
	}
	var tsState any
	if err := json.Unmarshal([]byte(g.StateText), &tsState); err != nil {
		t.Fatal(err)
	}
	if d := diff("model.json", generic(t, json.RawMessage(got)), tsState); d != "" {
		t.Fatal(d)
	}
	var s State
	if err := json.Unmarshal(got, &s); err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("a state written by TypeScript must validate: %v", err)
	}
}
