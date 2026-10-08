package criterio

import (
	"strconv"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/jsfmt"
)

// TaskTypes are the task kinds, in model class order.
var TaskTypes = domain.TaskTypes

// Reason is a signal that pushed a decision, with its signed contribution.
type Reason struct {
	Signal string  `json:"signal"`
	Weight float64 `json:"weight"`
}

// Source says where a decision came from.
type Source = string

const (
	SourceRules Source = "rules"
	SourceModel Source = "model"
	SourceBlend Source = "blend"
)

// Decision is a typed answer with confidence P in [0, 1] and its most influential signals.
type Decision[T any] struct {
	Value  T        `json:"value"`
	P      float64  `json:"p"`
	Why    []Reason `json:"why"`
	Source Source   `json:"source"`
}

// Action is what a decision would trigger; sensitive actions always need human confirmation.
type Action = string

const (
	ActionLabel                Action = "label"
	ActionOrder                Action = "order"
	ActionSuggestReview        Action = "suggest_review"
	ActionHardenClassification Action = "harden_classification"
	ActionSendData             Action = "send_data"
	ActionSwitchProfile        Action = "switch_profile"
)

// Disposition is auto, confirm or escalate.
type Disposition = string

// TaskSignals are text and repository facts about a task. Everything is optional.
type TaskSignals struct {
	Title         string
	Text          string
	CriteriaCount *int
	Diff          *DiffSignals
}

// DiffSignals are the facts of a diff.
type DiffSignals struct {
	FilesChanged int
	LinesAdded   int
	LinesDeleted int
	Paths        []string
}

// RunSignals are facts about a finished run, for "did it meet the brief?".
type RunSignals struct {
	State             string
	ResultText        string
	PermissionDenials int
	CriteriaCount     *int
	Diff              *DiffSignals
}

func itoa(i int) string { return strconv.Itoa(i) }

// num formats a number the way a JS template literal would.
func num(f float64) string { return jsfmt.Number(f) }

func typeIndex(t string) int {
	for i, x := range TaskTypes {
		if x == t {
			return i
		}
	}
	return -1
}
