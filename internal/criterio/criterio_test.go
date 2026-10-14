package criterio

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

func task(title string) TaskSignals { return TaskSignals{Title: title} }

func TestTokenize(t *testing.T) {
	for _, s := range []string{"Refactorizar la documentación", "refactor the documentation"} {
		if got := Tokenize(s); !reflect.DeepEqual(got, []string{"refact", "docume"}) {
			t.Errorf("%q → %v", s, got)
		}
	}
}

func TestTaskTypeRulesColdStart(t *testing.T) {
	c := New()
	cases := map[string]string{
		"Refactorizar el módulo de pagos":             "refactor",
		"Corregir bug en el login":                    "fix",
		"Documentar la API en el README":              "docs",
		"Add support for SSH destinations":            "implement",
		"Investigar alternativas de UI para terminal": "research",
		"Aumentar la cobertura de tests unitarios":    "test",
		"Revisar el PR de autenticación":              "review",
	}
	for title, want := range cases {
		d := c.TaskType(task(title))
		if d.Value != want || d.Source != SourceRules || len(d.Why) == 0 {
			t.Errorf("%q → %s (%s, %d reasons), want %s", title, d.Value, d.Source, len(d.Why), want)
		}
	}
}

func TestLearnsFromCorrections(t *testing.T) {
	c := New()
	if c.TaskType(task("Spike de tuberías WSL")).Value == "research" {
		t.Fatal("precondition: rules alone do not know 'spike'")
	}
	for i := 0; i < 12; i++ {
		c.LearnTaskType(task("Spike de "+[]string{"sockets", "colas", "caché", "hilos"}[i%4]), "research")
		c.LearnTaskType(task("Implementar "+[]string{"login", "logout", "perfil"}[i%3]), "implement")
	}
	after := c.TaskType(task("Spike de tuberías WSL"))
	if after.Value != "research" || after.Source == SourceRules {
		t.Fatalf("after learning: %s (%s)", after.Value, after.Source)
	}
	found := false
	for _, r := range after.Why {
		found = found || strings.Contains(r.Signal, "spike")
	}
	if !found {
		t.Errorf("reasons do not mention the learned word: %v", after.Why)
	}
}

func TestDoesNotTiltUnrelatedTasks(t *testing.T) {
	c := New()
	for _, w := range []string{"colas", "sockets", "caché"} {
		c.LearnTaskType(task("Spike de "+w), "research")
	}
	if v := c.TaskType(task("Corregir bug en el login")).Value; v != "fix" {
		t.Errorf("fix became %s", v)
	}
	if v := c.TaskType(task("Exportar reporte mensual")).Value; v == "research" {
		t.Error("unrelated task tilted to research")
	}
}

func TestComplexityLearning(t *testing.T) {
	c := New()
	s := task("Ajustar el logo")
	base := c.Complexity(s, "implement").Value
	for i := 0; i < 5; i++ {
		c.LearnComplexity(s, "implement", 9)
	}
	if got := c.Complexity(s, "implement").Value; got <= base+2 {
		t.Errorf("complexity %v, base %v", got, base)
	}
}

func TestSensitivityHardensOnly(t *testing.T) {
	c := New()
	s := TaskSignals{Title: "Exportar datos de clientes desde Jira interno", Text: "Contrato confidencial"}
	if !c.Sensitive(s).Value {
		t.Error("corporate brief not flagged")
	}
	if v := c.SuggestClassification(s, "personal", nil).Value; v != "corporate" {
		t.Errorf("suggested %q", v)
	}
	if v := c.SuggestClassification(task("Exportar datos de clientes desde Jira interno"), "confidential", nil).Value; v != "" {
		t.Errorf("relaxed to %q", v)
	}
	if v := c.SuggestClassification(task("Cambiar color del botón"), "personal", nil).Value; v != "" {
		t.Errorf("harmless brief hardened to %q", v)
	}
}

func TestDiffRisk(t *testing.T) {
	c := New()
	risky := c.DiffRisk(DiffSignals{FilesChanged: 12, LinesAdded: 600, LinesDeleted: 900, Paths: []string{".env", ".github/workflows/ci.yml", "src/auth/session.ts"}})
	calm := c.DiffRisk(DiffSignals{FilesChanged: 1, LinesAdded: 5, LinesDeleted: 1, Paths: []string{"README.md"}})
	if risky.Value < 8 || calm.Value > 3 {
		t.Errorf("risky %v calm %v", risky.Value, calm.Value)
	}
}

func TestAcceptance(t *testing.T) {
	c := New()
	if c.MeetsCriteria(RunSignals{State: "blocked"}).Value {
		t.Error("blocked run accepted")
	}
	if !c.MeetsCriteria(RunSignals{State: "completed", ResultText: "Listo, tests en verde"}).Value {
		t.Error("clean run rejected")
	}
	if c.MeetsCriteria(RunSignals{State: "completed", ResultText: "I could not run the tests", PermissionDenials: 2}).Value {
		t.Error("doubtful run accepted")
	}
}

func TestDisposition(t *testing.T) {
	cases := []struct {
		p      float64
		action Action
		risk   float64
		want   Disposition
	}{
		{0.99, ActionSendData, 0, "confirm"},
		{0.99, ActionSwitchProfile, 0, "confirm"},
		{0.99, ActionLabel, 0, "auto"},
		{0.7, ActionLabel, 0, "confirm"},
		{0.4, ActionLabel, 0, "confirm"},
		{0.4, ActionSuggestReview, 0, "escalate"},
		{0.99, ActionHardenClassification, 0, "confirm"},
		{0.99, ActionLabel, 9, "escalate"},
	}
	for _, c := range cases {
		if got := DispositionFor(c.p, c.action, c.risk, DefaultThresholds); got != c.want {
			t.Errorf("%v %s risk %v: %s, want %s", c.p, c.action, c.risk, got, c.want)
		}
	}
}

func headroom(x float64) *float64 { return &x }

func quotaWeight(r Ranked) float64 {
	for _, w := range r.Why {
		if strings.HasPrefix(w.Signal, "margen") {
			return w.Weight
		}
	}
	return 0
}

func TestRanking(t *testing.T) {
	c := New()
	for i := 0; i < 15; i++ {
		c.RecordOutcome("codex", "refactor", true)
		c.RecordOutcome("claude", "refactor", i%3 == 0)
	}
	ranked := c.RankProfiles([]Candidate{{ID: "claude", App: "claude-code"}, {ID: "codex", App: "codex"}}, "refactor", 42)
	if ranked[0].ID != "codex" || !regexp.MustCompile(`% de éxito en 15 tareas refactor`).MatchString(ranked[0].Why[0].Signal) {
		t.Errorf("ranking: %+v", ranked[0])
	}
	cands := []Candidate{{ID: "a", App: "x"}, {ID: "b", App: "y"}, {ID: "c", App: "z"}}
	if !reflect.DeepEqual(c.RankProfiles(cands, "fix", 7), c.RankProfiles(cands, "fix", 7)) {
		t.Error("ranking is not deterministic for a seed")
	}
	same := c.RankProfiles([]Candidate{
		{ID: "claude-a", App: "claude-code", QuotaHeadroom: headroom(0.95)},
		{ID: "claude-b", App: "claude-code", QuotaHeadroom: headroom(0.05)},
	}, "implement", 1)
	if quotaWeight(same[0]) != quotaWeight(same[1]) {
		t.Error("quota separated profiles of the same app (FR-012)")
	}
	apps := c.RankProfiles([]Candidate{
		{ID: "claude", App: "claude-code", QuotaHeadroom: headroom(0.05)},
		{ID: "codex", App: "codex", QuotaHeadroom: headroom(0.95)},
	}, "implement", 3)
	q := map[string]float64{}
	for _, r := range apps {
		q[r.ID] = quotaWeight(r)
	}
	if q["codex"] <= q["claude"] {
		t.Errorf("quota between apps: %v", q)
	}
}

func TestStatistics(t *testing.T) {
	if _, ok := Brier(nil); ok {
		t.Error("brier of nothing")
	}
	if s, _ := Brier([]Outcome{{1, 1}, {0, 0}}); s != 0 {
		t.Error("perfect brier")
	}
	if s, _ := Brier([]Outcome{{0.5, 1}}); s != 0.25 {
		t.Error("coin flip brier")
	}
	if Reliability([]Outcome{{0.9, 1}}, 5)[4].Count != 1 {
		t.Error("reliability last bin")
	}
	rng := Mulberry32(1)
	sum := 0.0
	for i := 0; i < 4000; i++ {
		sum += SampleBeta(8, 2, rng)
	}
	if mean := sum / 4000; mean < 0.77 || mean > 0.83 {
		t.Errorf("Beta(8,2) mean %v", mean)
	}
}

func TestPerformance(t *testing.T) {
	c := New()
	for i := 0; i < 50; i++ {
		c.LearnTaskType(task("Implementar cosa "+itoa(i)), "implement")
	}
	text := "Refactorizar el adaptador de Codex para normalizar eventos JSONL y añadir pruebas"
	n := 2000
	start := time.Now()
	for i := 0; i < n; i++ {
		c.TaskType(task(text + " " + itoa(i)))
	}
	if per := time.Since(start) / time.Duration(n); per > 5*time.Millisecond {
		t.Errorf("%v per decision (NFR-013 budget: 5 ms)", per)
	}
}

func TestStateHasNoText(t *testing.T) {
	c := New()
	c.LearnTaskType(task("Spike secreto proyecto Atlas"), "research")
	c.RecordOutcome("p1", "research", true)
	raw, _ := json.Marshal(c.State())
	if regexp.MustCompile(`(?i)atlas|secret|spike`).Match(raw) {
		t.Fatalf("state leaks text: %s", raw)
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil || s.Validate() != nil {
		t.Fatalf("round trip: %v %v", err, s.Validate())
	}
	again, _ := json.Marshal(FromState(s).State())
	if string(again) != string(raw) {
		t.Error("state changed after a round trip")
	}
}

func TestInvalidStateIsRejected(t *testing.T) {
	for _, doc := range []string{
		`{"version":1,"taskType":42}`,
		`{"version":2}`,
		`{"version":1,"taskType":{"classes":7,"weights":{"x":[1]},"updates":0}}`,
	} {
		var s State
		if json.Unmarshal([]byte(doc), &s) == nil && s.Validate() == nil {
			t.Errorf("accepted %s", doc)
		}
	}
}

func TestTokensMatchTypeScript(t *testing.T) {
	raw, err := os.ReadFile("../jsfmt/testdata/text.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Tokens []struct {
			Text   string   `json:"text"`
			Tokens []string `json:"tokens"`
			Fnv    uint32   `json:"fnv"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.Tokens {
		if got := Tokenize(c.Text); !reflect.DeepEqual(got, c.Tokens) || FNV1a(c.Text) != c.Fnv {
			t.Errorf("%q: tokens %v/%v fnv %d/%d", c.Text, got, c.Tokens, FNV1a(c.Text), c.Fnv)
		}
	}
}

// A research brief that asks for details and sources is research, not docs (fixed 2026-10-13).
func TestResearchBriefWithSourcesIsResearch(t *testing.T) {
	probs, _ := TaskTypeRules(TaskSignals{
		Title: "Top 10 OWASP LLM",
		Text:  "Dime el detalle del top 10 de OWASP para aplicaciones LLM, con fuentes oficiales.",
	})
	best := 0
	for i, p := range probs {
		if p > probs[best] {
			best = i
		}
	}
	if TaskTypes[best] != "research" {
		t.Errorf("type %s (%v), want research", TaskTypes[best], probs)
	}
}
