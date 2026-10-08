package criterio

import (
	"math"
	"sort"
)

const (
	// blendK is the number of examples before the learned model weighs as much as the rules.
	blendK     = 10
	historyMax = 500
	// toolsBlendK: tool confirmations are explicit labels, so the model catches up sooner.
	toolsBlendK = 5
	// maxToolModels bounds the per-tool models kept (bounded state, anti-abuse).
	maxToolModels = 200
)

// Question names what was asked, for calibration history.
type Question = string

const (
	QTaskType    Question = "taskType"
	QSensitivity Question = "sensitivity"
	QAcceptance  Question = "acceptance"
	QTools       Question = "tools"
)

// HistoryEntry is a past prediction and whether it was right.
type HistoryEntry struct {
	Q Question `json:"q"`
	P float64  `json:"p"`
	Y int      `json:"y"`
}

// ComplexityOffset is the learned complexity correction for one task type.
type ComplexityOffset struct {
	Sum float64 `json:"sum"`
	N   int     `json:"n"`
}

// Candidate is a profile that could run a task.
type Candidate struct {
	ID string
	// App is used so quota headroom never separates profiles of the same app (FR-012).
	App        string
	Preference int
	// QuotaHeadroom is the remaining share of the tightest known window, 0..1; nil when unknown.
	QuotaHeadroom *float64
}

// Ranked is a candidate with its score and reasons.
type Ranked struct {
	ID    string   `json:"id"`
	Score float64  `json:"score"`
	Why   []Reason `json:"why"`
}

// ToolDecision is the per-tool part of RequiredTools.
type ToolDecision struct {
	Tool string   `json:"tool"`
	P    float64  `json:"p"`
	Why  []Reason `json:"why"`
}

// ToolsDecision is the answer to "which tools does this brief need?".
type ToolsDecision struct {
	Decision[[]string]
	PerTool []ToolDecision `json:"perTool"`
}

// Criterio is the engine. It is deterministic given its state and seed.
type Criterio struct {
	taskType    *OnlineSoftmax
	sensitivity *OnlineSoftmax
	acceptance  *OnlineSoftmax
	tools       *ordered[*OnlineSoftmax]
	complexity  *ordered[ComplexityOffset]
	arms        *ordered[BetaArm]
	history     []HistoryEntry
	Thresholds  Thresholds
}

// New returns an engine with no learned state (rules only).
func New() *Criterio {
	return &Criterio{
		taskType:    NewOnlineSoftmax(len(TaskTypes)),
		sensitivity: NewOnlineSoftmax(2),
		acceptance:  NewOnlineSoftmax(2),
		tools:       newOrdered[*OnlineSoftmax](),
		complexity:  newOrdered[ComplexityOffset](),
		arms:        newOrdered[BetaArm](),
		history:     []HistoryEntry{},
		Thresholds:  DefaultThresholds,
	}
}

func blend(rules, model []float64, n, k int) (probs []float64, alpha float64) {
	alpha = float64(n) / float64(n+k)
	probs = make([]float64, len(rules))
	for i, r := range rules {
		m := 0.0
		if i < len(model) {
			m = model[i]
		}
		probs[i] = (1-alpha)*r + alpha*m
	}
	return probs, alpha
}

func argmax(xs []float64) int {
	best := 0
	for i := 1; i < len(xs); i++ {
		if xs[i] > xs[best] {
			best = i
		}
	}
	return best
}

func mergeWhy(a, b []Reason, top int) []Reason {
	all := append(append([]Reason{}, a...), b...)
	all = sortByAbsWeightDesc(all)
	if len(all) > top {
		all = all[:top]
	}
	return all
}

func head(r []Reason, n int) []Reason {
	if len(r) > n {
		return r[:n]
	}
	return r
}

func sourceOf(alpha float64) Source {
	switch {
	case alpha == 0:
		return SourceRules
	case alpha > 0.95:
		return SourceModel
	}
	return SourceBlend
}

func runFeatures(r RunSignals) []Feature {
	feats := []Feature{{Name: "bias", Value: 1}, {Name: "state:" + r.State, Value: 1}}
	toks := Tokenize(r.ResultText)
	if len(toks) > 200 {
		toks = toks[:200]
	}
	for _, t := range unique(toks) {
		feats = append(feats, Feature{Name: "r:" + t, Value: 1})
	}
	if r.PermissionDenials != 0 {
		feats = append(feats, Feature{Name: "denials", Value: float64(min(r.PermissionDenials, 5))})
	}
	if r.Diff != nil {
		feats = append(feats, DiffFeatures(*r.Diff)...)
	}
	return feats
}

func (c *Criterio) predictIf(m *OnlineSoftmax, feats []Feature) []float64 {
	if m.Updates() == 0 {
		return nil
	}
	return m.Predict(feats)
}

func (c *Criterio) explainIf(m *OnlineSoftmax, feats []Feature, class, top int) []Reason {
	if m.Updates() == 0 {
		return nil
	}
	return m.Explain(feats, class, top)
}

// TaskType answers "what kind of task is this?".
func (c *Criterio) TaskType(s TaskSignals) Decision[string] {
	rules, rulesWhy := TaskTypeRules(s)
	feats := TaskFeatures(s)
	n := c.taskType.Updates()
	probs, alpha := blend(rules, c.predictIf(c.taskType, feats), n, blendK)
	i := argmax(probs)
	why := mergeWhy(head(rulesWhy, 4), c.explainIf(c.taskType, feats, i, 4), 5)
	return Decision[string]{Value: TaskTypes[i], P: probs[i], Why: why, Source: sourceOf(alpha)}
}

// Complexity scores a task 2..10. taskType may be "" when unknown.
func (c *Criterio) Complexity(s TaskSignals, taskType string) Decision[float64] {
	score, rulesWhy := ComplexityRules(s)
	off, hasOff := ComplexityOffset{}, false
	if taskType != "" {
		off, hasOff = c.complexity.get(taskType)
	}
	shift := 0.0
	if hasOff && off.N != 0 {
		shift = off.Sum / float64(off.N)
	}
	words := len(Tokenize(s.Title + " " + s.Text))
	why := append([]Reason{}, rulesWhy...)
	if math.Abs(shift) >= 0.2 {
		why = append(why, Reason{Signal: "ajuste aprendido para tareas " + taskType, Weight: shift})
	}
	src := SourceRules
	if hasOff && off.N != 0 {
		src = SourceBlend
	}
	return Decision[float64]{
		Value:  ClampScore(score + shift),
		P:      math.Min(0.9, 0.45+0.45*math.Min(1, float64(words)/40)),
		Why:    why,
		Source: src,
	}
}

// DiffRisk scores a diff 2..10. Facts-based, so confidence is high.
func (c *Criterio) DiffRisk(d DiffSignals) Decision[float64] {
	score, why := DiffRiskRules(d)
	return Decision[float64]{Value: score, P: 0.8, Why: why, Source: SourceRules}
}

func (c *Criterio) binary(m *OnlineSoftmax, ruleP float64, ruleWhy []Reason, feats []Feature) Decision[bool] {
	n := m.Updates()
	probs, alpha := blend([]float64{1 - ruleP, ruleP}, c.predictIf(m, feats), n, blendK)
	p1 := probs[1]
	value := p1 >= 0.5
	class, p := 0, 1-p1
	if value {
		class, p = 1, p1
	}
	return Decision[bool]{Value: value, P: p, Why: mergeWhy(ruleWhy, c.explainIf(m, feats, class, 4), 5), Source: sourceOf(alpha)}
}

// Sensitive answers "does the brief look corporate or personal-sensitive?" (only to harden).
func (c *Criterio) Sensitive(s TaskSignals) Decision[bool] {
	p, why := SensitivityRules(s)
	return c.binary(c.sensitivity, p, why, TaskFeatures(s))
}

// SuggestClassification proposes a stricter classification when the brief looks sensitive,
// never a less strict one. order goes from least to most strict; nil means the defaults.
// The value is "" when no change is suggested.
func (c *Criterio) SuggestClassification(s TaskSignals, current string, order []string) Decision[string] {
	if order == nil {
		order = []string{"public", "personal", "corporate", "confidential"}
	}
	d := c.Sensitive(s)
	target := current
	if contains(order, "corporate") {
		target = "corporate"
	} else if len(order) > 0 {
		target = order[len(order)-1]
	}
	value := ""
	if d.Value && indexOf(order, current) < indexOf(order, target) {
		value = target
	}
	return Decision[string]{Value: value, P: d.P, Why: d.Why, Source: d.Source}
}

// MeetsCriteria answers "did the run satisfy the brief?".
func (c *Criterio) MeetsCriteria(r RunSignals) Decision[bool] {
	p, why := AcceptanceRules(r)
	return c.binary(c.acceptance, p, why, runFeatures(r))
}

type toolEval struct {
	tool  string
	p     float64
	why   []Reason
	alpha float64
}

func (c *Criterio) evalTools(s TaskSignals, catalog ToolCatalog) []toolEval {
	text := s.Title + " " + s.Text
	feats := TaskFeatures(s)
	out := make([]toolEval, 0, len(catalog))
	for _, e := range catalog {
		ruleP, ruleWhy := toolRule(text, e.Aliases)
		model, ok := c.tools.get(e.Tool)
		n := 0
		var pred []float64
		var learned []Reason
		if ok {
			n = model.Updates()
			pred = c.predictIf(model, feats)
			learned = c.explainIf(model, feats, 1, 2)
		}
		probs, alpha := blend([]float64{1 - ruleP, ruleP}, pred, n, toolsBlendK)
		out = append(out, toolEval{tool: e.Tool, p: probs[1], why: mergeWhy(ruleWhy, learned, 3), alpha: alpha})
	}
	return out
}

// RequiredTools answers which catalog tools a brief needs (FR-021). It affects routing, so the
// answer is always confirmed by the user.
func (c *Criterio) RequiredTools(s TaskSignals, catalog ToolCatalog) ToolsDecision {
	per := c.evalTools(s, catalog)
	chosen := []string{}
	why := []Reason{}
	conf := 1.0
	src := SourceRules
	perTool := make([]ToolDecision, len(per))
	for i, t := range per {
		perTool[i] = ToolDecision{Tool: t.tool, P: t.p, Why: t.why}
		// Confidence of the whole answer: product of each tool's own decision confidence.
		conf *= math.Max(t.p, 1-t.p)
		if t.alpha > 0 {
			src = SourceBlend
		}
		if t.p >= 0.5 {
			chosen = append(chosen, t.tool)
			for _, w := range t.why {
				why = append(why, Reason{Signal: t.tool + ": " + w.Signal, Weight: w.Weight})
			}
		}
	}
	return ToolsDecision{
		Decision: Decision[[]string]{Value: chosen, P: conf, Why: head(why, 5), Source: src},
		PerTool:  perTool,
	}
}

// LearnRequiredTools records the user's confirmation of the tools a brief needs.
func (c *Criterio) LearnRequiredTools(s TaskSignals, catalog ToolCatalog, chosen []string) {
	predicted := c.evalTools(s, catalog)
	feats := TaskFeatures(s)
	for _, t := range predicted {
		y := 0
		if contains(chosen, t.tool) {
			y = 1
		}
		guess := 0
		p := 1 - t.p
		if t.p >= 0.5 {
			guess, p = 1, t.p
		}
		right := guess == y
		c.record(QTools, p, b2i(right))
		model, ok := c.tools.get(t.tool)
		if !ok {
			if c.tools.len() >= maxToolModels {
				continue
			}
			model = NewOnlineSoftmax(2)
			c.tools.set(t.tool, model)
		}
		weight := 2.0
		if right {
			weight = 1
		}
		model.Update(feats, y, weight)
	}
}

// Dispose says what Hilvan may do with a decision on its own.
func (c *Criterio) Dispose(p float64, action Action, risk float64) Disposition {
	return DispositionFor(p, action, risk, c.Thresholds)
}

// RankProfiles orders eligible profiles for a task type: Thompson sample of past success,
// quota headroom (equalised within each app) and user preference.
func (c *Criterio) RankProfiles(candidates []Candidate, taskType string, seed uint32) []Ranked {
	rng := Mulberry32(seed)
	byApp := map[string][]float64{}
	for _, cand := range candidates {
		if cand.QuotaHeadroom != nil {
			byApp[cand.App] = append(byApp[cand.App], *cand.QuotaHeadroom)
		}
	}
	pMin, pMax := math.Inf(1), math.Inf(-1)
	for _, cand := range candidates {
		pMin = math.Min(pMin, float64(cand.Preference))
		pMax = math.Max(pMax, float64(cand.Preference))
	}
	pSpan := pMax - pMin
	if pSpan == 0 || math.IsNaN(pSpan) {
		pSpan = 1
	}
	out := make([]Ranked, 0, len(candidates))
	for _, cand := range candidates {
		arm, ok := c.arms.get(cand.ID + "|" + taskType)
		if !ok {
			arm = Prior
		}
		sample := SampleBeta(arm.A, arm.B, rng)
		heads, reported := byApp[cand.App]
		// Same-app profiles share the app's mean headroom: quota cannot pick between them.
		quota := 0.5
		if reported {
			sum := 0.0
			for _, h := range heads {
				sum += h
			}
			quota = sum / float64(len(heads))
		}
		pref := (float64(cand.Preference) - pMin) / pSpan
		score := 0.6*sample + 0.25*quota + 0.15*pref
		history := "sin historial en tareas " + taskType + " (exploración)"
		if obs := arm.Observations(); obs != 0 {
			history = num(roundJS(arm.Mean()*100)) + " % de éxito en " + num(obs) + " tareas " + taskType
		}
		quotaText := "cuota no reportada"
		if reported {
			quotaText = "margen de cuota de " + cand.App + ": " + num(roundJS(quota*100)) + " %"
		}
		why := []Reason{{Signal: history, Weight: 0.6 * sample}, {Signal: quotaText, Weight: 0.25 * quota}}
		if cand.Preference != 0 {
			why = append(why, Reason{Signal: "preferencia " + itoa(cand.Preference), Weight: 0.15 * pref})
		}
		out = append(out, Ranked{ID: cand.ID, Score: score, Why: why})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func (c *Criterio) record(q Question, p float64, y int) {
	c.history = append(c.history, HistoryEntry{Q: q, P: p, Y: y})
	if len(c.history) > historyMax {
		c.history = append([]HistoryEntry{}, c.history[len(c.history)-historyMax:]...)
	}
}

// LearnTaskType records the user's confirmation or correction (corrections count double).
func (c *Criterio) LearnTaskType(s TaskSignals, label string) {
	predicted := c.TaskType(s)
	corrected := predicted.Value != label
	c.record(QTaskType, predicted.P, b2i(!corrected))
	weight := 1.0
	if corrected {
		weight = 2
	}
	c.taskType.Update(TaskFeatures(s), typeIndex(label), weight)
}

// LearnComplexity shifts complexity towards the user's score for a task type.
func (c *Criterio) LearnComplexity(s TaskSignals, taskType string, userScore float64) {
	rule, _ := ComplexityRules(s)
	cur, _ := c.complexity.get(taskType)
	delta := math.Max(-4, math.Min(4, userScore-rule))
	c.complexity.set(taskType, ComplexityOffset{Sum: cur.Sum + delta, N: cur.N + 1})
}

// LearnSensitivity records whether a brief was sensitive.
func (c *Criterio) LearnSensitivity(s TaskSignals, sensitive bool) {
	d := c.Sensitive(s)
	right := d.Value == sensitive
	c.record(QSensitivity, d.P, b2i(right))
	weight := 2.0
	if right {
		weight = 1
	}
	c.sensitivity.Update(TaskFeatures(s), b2i(sensitive), weight)
}

// LearnAcceptance records whether a run's result was accepted.
func (c *Criterio) LearnAcceptance(r RunSignals, accepted bool) {
	d := c.MeetsCriteria(r)
	c.record(QAcceptance, d.P, b2i(d.Value == accepted))
	c.acceptance.Update(runFeatures(r), b2i(accepted), 1)
}

// RecordOutcome feeds the profile ranking with the outcome of a task run by a profile.
func (c *Criterio) RecordOutcome(profileID, taskType string, success bool) {
	key := profileID + "|" + taskType
	arm, ok := c.arms.get(key)
	if !ok {
		arm = Prior
	}
	c.arms.set(key, arm.update(success, 1))
}

// ProfileStat is the success estimate of one profile and task type.
type ProfileStat struct {
	Key  string  `json:"key"`
	Mean float64 `json:"mean"`
	N    float64 `json:"n"`
}

// Stats summarises what Criterio has learned and how well calibrated it is.
type Stats struct {
	Examples    map[Question]int      `json:"examples"`
	Brier       *float64              `json:"brier"`
	ByQuestion  map[Question]*float64 `json:"byQuestion"`
	Reliability []ReliabilityBin      `json:"reliability"`
	Profiles    []ProfileStat         `json:"profiles"`
}

func brierPtr(o []Outcome) *float64 {
	if s, ok := Brier(o); ok {
		return &s
	}
	return nil
}

// Stats returns the learning and calibration summary.
func (c *Criterio) Stats() Stats {
	all := make([]Outcome, len(c.history))
	byQ := map[Question][]Outcome{}
	for i, h := range c.history {
		all[i] = Outcome{P: h.P, Y: h.Y}
		byQ[h.Q] = append(byQ[h.Q], all[i])
	}
	toolUpdates := 0
	for _, k := range c.tools.keys {
		toolUpdates += c.tools.m[k].Updates()
	}
	profiles := []ProfileStat{}
	for _, k := range c.arms.keys {
		arm := c.arms.m[k]
		profiles = append(profiles, ProfileStat{Key: k, Mean: arm.Mean(), N: arm.Observations()})
	}
	return Stats{
		Examples: map[Question]int{
			QTaskType: c.taskType.Updates(), QSensitivity: c.sensitivity.Updates(),
			QAcceptance: c.acceptance.Updates(), QTools: toolUpdates,
		},
		Brier: brierPtr(all),
		ByQuestion: map[Question]*float64{
			QTaskType: brierPtr(byQ[QTaskType]), QSensitivity: brierPtr(byQ[QSensitivity]),
			QAcceptance: brierPtr(byQ[QAcceptance]), QTools: brierPtr(byQ[QTools]),
		},
		Reliability: Reliability(all, 5),
		Profiles:    profiles,
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func contains(xs []string, v string) bool { return indexOf(xs, v) >= 0 }

func indexOf(xs []string, v string) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}
