package core

import (
	"sort"

	"github.com/ferchosur/hilvan/internal/domain"
)

// ProfileUsage is per-profile usage built only from what the CLIs reported (FR-016):
// estimates, never balances.
type ProfileUsage struct {
	ProfileID string         `json:"profileId"`
	Runs      int            `json:"runs"`
	States    map[string]int `json:"states"`
	// StateOrder lists States keys in first-seen order (how the TypeScript object enumerates).
	StateOrder   []string  `json:"-"`
	CostUSD      float64   `json:"costUsd"`
	CostRuns     int       `json:"costRuns"`
	InputTokens  int64     `json:"inputTokens"`
	OutputTokens int64     `json:"outputTokens"`
	LastLimits   *LimitsAt `json:"lastLimits,omitempty"`
	Accepted     int       `json:"accepted"`
	Discarded    int       `json:"discarded"`
	// Reviews counts cross reviews performed by this profile (their usage is included above).
	Reviews int `json:"reviews"`
}

// LimitsAt are the latest quota windows reported for a profile.
type LimitsAt struct {
	At      string               `json:"at"`
	Windows []domain.LimitWindow `json:"windows"`
}

func addUsage(u *ProfileUsage, usage *domain.Usage) {
	if usage == nil {
		return
	}
	if usage.CostUSD != nil {
		u.CostUSD += *usage.CostUSD
		u.CostRuns++
	}
	if usage.InputTokens != nil {
		u.InputTokens += *usage.InputTokens
	}
	if usage.OutputTokens != nil {
		u.OutputTokens += *usage.OutputTokens
	}
}

// UsageByProfile aggregates runs, reviews and outcomes per profile, most active first.
func UsageByProfile(tasks []domain.Task) []ProfileUsage {
	var order []*ProfileUsage
	by := map[string]*ProfileUsage{}
	get := func(id string) *ProfileUsage {
		u, ok := by[id]
		if !ok {
			u = &ProfileUsage{ProfileID: id, States: map[string]int{}}
			by[id] = u
			order = append(order, u)
		}
		return u
	}
	for _, t := range tasks {
		for _, r := range t.Runs {
			u := get(r.ProfileID)
			u.Runs++
			if u.States[r.State] == 0 {
				u.StateOrder = append(u.StateOrder, r.State)
			}
			u.States[r.State]++
			addUsage(u, r.Usage)
			if len(r.Limits) > 0 && (u.LastLimits == nil || r.StartedAt > u.LastLimits.At) {
				u.LastLimits = &LimitsAt{At: r.StartedAt, Windows: r.Limits}
			}
		}
		for _, r := range t.Reviews {
			u := get(r.ProfileID)
			u.Reviews++
			addUsage(u, r.Usage)
		}
		if t.ProfileID != nil && (t.State == domain.TaskAccepted || t.State == domain.TaskDiscarded) {
			u := get(*t.ProfileID)
			if t.State == domain.TaskAccepted {
				u.Accepted++
			} else {
				u.Discarded++
			}
		}
	}
	out := make([]ProfileUsage, len(order))
	for i, u := range order {
		out[i] = *u
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Runs > out[j].Runs })
	return out
}
