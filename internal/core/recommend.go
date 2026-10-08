package core

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ferchosur/hilvan/internal/criterio"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/policy"
)

// Ranked is an eligible profile with its score and reasons.
type Ranked struct {
	Profile    *domain.Profile
	Evaluation policy.Evaluation
	Score      float64
	Why        []criterio.Reason
}

// Excluded is a profile that cannot take the task, with its evaluation.
type Excluded struct {
	Profile    *domain.Profile
	Evaluation policy.Evaluation
}

// Recommendation lists eligible profiles (best first) and the excluded ones with reasons.
type Recommendation struct {
	TaskType string
	Ranked   []Ranked
	Excluded []Excluded
}

// QuotaHeadroom is the remaining share of the tightest window reported by the profile's latest
// run with limits (FR-012: only data the provider exposes). A recent rate-limit block counts as
// zero. ok is false when nothing was reported.
func QuotaHeadroom(profileID string, tasks []domain.Task, now time.Time) (float64, bool) {
	var runs []domain.RunSummary
	for _, t := range tasks {
		for _, r := range t.Runs {
			if r.ProfileID == profileID {
				runs = append(runs, r)
			}
		}
	}
	if len(runs) == 0 {
		return 0, false
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].StartedAt > runs[j].StartedAt })
	if latest := runs[0]; latest.BlockedReason == "rate_limit" {
		if at, err := time.Parse(time.RFC3339Nano, latest.StartedAt); err == nil && now.Sub(at) < 5*time.Hour {
			return 0, true
		}
	}
	for _, r := range runs {
		if len(r.Limits) > 0 {
			max := math.Inf(-1)
			for _, w := range r.Limits {
				max = math.Max(max, w.Utilization)
			}
			return 1 - max, true
		}
	}
	return 0, false
}

// Recommend checks eligibility first; Criterio then ranks only the eligible profiles (D-008,
// D-009). Reserved profiles only take tasks that need one of their tools, unless nothing else
// is eligible (FR-021).
func Recommend(pol domain.EffectivePolicy, t domain.Task, profiles []domain.Profile, c *criterio.Criterio, history []domain.Task, seed uint32) Recommendation {
	taskType := t.TaskType
	if taskType == "" {
		taskType = c.TaskType(criterio.TaskSignals{Title: t.Title}).Value
	}
	facts := policy.TaskFacts{Classification: t.Classification, Requires: t.Requires}
	type eval struct {
		p  *domain.Profile
		ev policy.Evaluation
	}
	var evals, eligibleAll, defaults, reservedOut []eval
	needsOwnTool := func(p *domain.Profile) bool {
		for _, r := range t.Requires {
			if contains(p.Tools.Provides, r) {
				return true
			}
		}
		return false
	}
	for i := range profiles {
		p := &profiles[i]
		e := eval{p, policy.Evaluate(pol, facts, p, nil)}
		evals = append(evals, e)
		if e.ev.Eligible {
			eligibleAll = append(eligibleAll, e)
			if p.Routing != "reserved" || needsOwnTool(p) {
				defaults = append(defaults, e)
			} else {
				reservedOut = append(reservedOut, e)
			}
		}
	}
	eligible := defaults
	var routedOut []Excluded
	if len(defaults) == 0 {
		eligible = eligibleAll
	} else {
		for _, e := range reservedOut {
			tools := strings.Join(e.p.Tools.Provides, ", ")
			if tools == "" {
				tools = "its tools"
			}
			ev := e.ev
			ev.Eligible = false
			ev.Exclusions = []policy.Exclusion{{Reason: "reserved for tasks that need " + tools, Source: "routing"}}
			routedOut = append(routedOut, Excluded{e.p, ev})
		}
	}
	cands := make([]criterio.Candidate, len(eligible))
	byID := map[string]eval{}
	for i, e := range eligible {
		cands[i] = criterio.Candidate{ID: e.p.ID, App: e.p.App, Preference: e.p.Preference}
		if h, ok := QuotaHeadroom(e.p.ID, history, time.Now()); ok {
			cands[i].QuotaHeadroom = &h
		}
		byID[e.p.ID] = e
	}
	rec := Recommendation{TaskType: taskType}
	for _, r := range c.RankProfiles(cands, taskType, seed) {
		e := byID[r.ID]
		why := append([]criterio.Reason{}, r.Why...)
		if e.p.Routing == "reserved" {
			signal := "reservado, pero es el único perfil elegible"
			if needsOwnTool(e.p) {
				var own []string
				for _, x := range t.Requires {
					if contains(e.p.Tools.Provides, x) {
						own = append(own, x)
					}
				}
				signal = "reservado: la tarea requiere " + strings.Join(own, ", ")
			}
			why = append([]criterio.Reason{{Signal: signal, Weight: 1}}, why...)
		}
		rec.Ranked = append(rec.Ranked, Ranked{Profile: e.p, Evaluation: e.ev, Score: r.Score, Why: why})
	}
	for _, e := range evals {
		if !e.ev.Eligible {
			rec.Excluded = append(rec.Excluded, Excluded{e.p, e.ev})
		}
	}
	rec.Excluded = append(rec.Excluded, routedOut...)
	return rec
}
