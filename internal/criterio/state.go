package criterio

import (
	"fmt"
	"math"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/jsfmt"
)

func roundJS(x float64) float64 { return jsfmt.Round(x) }

// State is the persisted model (criterio/model.json). It stores hashed weights and counts,
// never the original text. Field order matches the TypeScript output.
type State struct {
	Version     int           `json:"version"`
	TaskType    SoftmaxState  `json:"taskType"`
	Sensitivity SoftmaxState  `json:"sensitivity"`
	Acceptance  SoftmaxState  `json:"acceptance"`
	Tools       *orderedTools `json:"tools,omitempty"`
	// ComplexityOffset is the learned complexity offset per task type.
	ComplexityOffset *ordered[ComplexityOffset] `json:"complexityOffset"`
	// Arms are success counts per "profileId|taskType".
	Arms    *ordered[BetaArm] `json:"arms"`
	History []HistoryEntry    `json:"history"`
}

type orderedTools = ordered[SoftmaxState]

// State serialises the engine.
func (c *Criterio) State() State {
	tools := newOrdered[SoftmaxState]()
	for _, k := range c.tools.keys {
		tools.set(k, c.tools.m[k].State())
	}
	complexity := newOrdered[ComplexityOffset]()
	for _, k := range c.complexity.keys {
		complexity.set(k, c.complexity.m[k])
	}
	arms := newOrdered[BetaArm]()
	for _, k := range c.arms.keys {
		arms.set(k, c.arms.m[k])
	}
	return State{
		Version:          1,
		TaskType:         c.taskType.State(),
		Sensitivity:      c.sensitivity.State(),
		Acceptance:       c.acceptance.State(),
		Tools:            tools,
		ComplexityOffset: complexity,
		Arms:             arms,
		History:          append([]HistoryEntry{}, c.history...),
	}
}

// FromState restores an engine from a validated state.
func FromState(s State) *Criterio {
	c := New()
	c.taskType = SoftmaxFromState(s.TaskType)
	c.sensitivity = SoftmaxFromState(s.Sensitivity)
	c.acceptance = SoftmaxFromState(s.Acceptance)
	if s.Tools != nil {
		for _, k := range s.Tools.keys {
			c.tools.set(k, SoftmaxFromState(s.Tools.m[k]))
		}
	}
	if s.ComplexityOffset != nil {
		for _, k := range s.ComplexityOffset.keys {
			c.complexity.set(k, s.ComplexityOffset.m[k])
		}
	}
	if s.Arms != nil {
		for _, k := range s.Arms.keys {
			c.arms.set(k, s.Arms.m[k])
		}
	}
	c.history = append([]HistoryEntry{}, s.History...)
	return c
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func validSoftmax(s SoftmaxState, classes int) error {
	if s.Classes < 2 || s.Classes > 64 || (classes > 0 && s.Classes != classes) {
		return fmt.Errorf("invalid number of classes %d", s.Classes)
	}
	if s.Updates < 0 {
		return fmt.Errorf("negative updates")
	}
	for _, row := range s.Weights {
		for _, x := range row {
			if !finite(x) {
				return fmt.Errorf("non-finite weight")
			}
		}
	}
	return nil
}

var questions = map[string]bool{QTaskType: true, QSensitivity: true, QAcceptance: true, QTools: true}

// Validate mirrors CriterioStateSchema: a corrupted or tampered file must not crash or skew Hilvan.
func (s State) Validate() error {
	if s.Version != 1 {
		return fmt.Errorf("unsupported version %d", s.Version)
	}
	if err := validSoftmax(s.TaskType, len(TaskTypes)); err != nil {
		return fmt.Errorf("taskType: %w", err)
	}
	if err := validSoftmax(s.Sensitivity, 2); err != nil {
		return fmt.Errorf("sensitivity: %w", err)
	}
	if err := validSoftmax(s.Acceptance, 2); err != nil {
		return fmt.Errorf("acceptance: %w", err)
	}
	if s.Tools != nil {
		if s.Tools.len() > maxToolModels {
			return fmt.Errorf("too many tool models")
		}
		for _, k := range s.Tools.keys {
			if !domain.IDPattern.MatchString(k) {
				return fmt.Errorf("invalid tool id %q", k)
			}
			if err := validSoftmax(s.Tools.m[k], 2); err != nil {
				return fmt.Errorf("tools.%s: %w", k, err)
			}
		}
	}
	if s.ComplexityOffset == nil || s.Arms == nil || s.History == nil {
		return fmt.Errorf("missing complexityOffset, arms or history")
	}
	for _, k := range s.ComplexityOffset.keys {
		o := s.ComplexityOffset.m[k]
		if typeIndex(k) < 0 || !finite(o.Sum) || o.N < 0 {
			return fmt.Errorf("invalid complexity offset %q", k)
		}
	}
	for _, k := range s.Arms.keys {
		a := s.Arms.m[k]
		if jsfmt.UTF16Len(k) > 200 || !(a.A > 0) || !(a.B > 0) || !finite(a.A) || !finite(a.B) {
			return fmt.Errorf("invalid arm %q", k)
		}
	}
	if len(s.History) > 1000 {
		return fmt.Errorf("history too long")
	}
	for _, h := range s.History {
		if !questions[h.Q] || h.P < 0 || h.P > 1 || (h.Y != 0 && h.Y != 1) {
			return fmt.Errorf("invalid history entry")
		}
	}
	return nil
}
