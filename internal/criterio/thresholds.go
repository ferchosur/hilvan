package criterio

// Thresholds map confidence and risk to what Hilvan may do on its own.
type Thresholds struct {
	// Auto: at or above, benign actions may run automatically.
	Auto float64
	// Escalate: below, escalate to the advisor or the user.
	Escalate float64
	// RiskEscalate: a risk score (2..10) at or above which any action escalates.
	RiskEscalate float64
}

// DefaultThresholds are Hilvan's defaults.
var DefaultThresholds = Thresholds{Auto: 0.85, Escalate: 0.55, RiskEscalate: 7.5}

var (
	// sensitiveActions move data between identities or change who runs a task (D-009, FR-020).
	sensitiveActions = map[Action]bool{ActionSendData: true, ActionSwitchProfile: true}
	// benignActions are visible and reversible: low confidence just asks the user.
	benignActions = map[Action]bool{ActionLabel: true, ActionOrder: true}
	// alwaysShown actions are shown even when confident (classification changes eligibility).
	alwaysShown = map[Action]bool{ActionHardenClassification: true}
)

// DispositionFor says whether an action may be automatic, needs confirmation or escalates.
// Sensitive actions are never auto, whatever the confidence.
func DispositionFor(p float64, action Action, risk float64, t Thresholds) Disposition {
	if risk >= t.RiskEscalate {
		return "escalate"
	}
	if p < t.Escalate {
		if benignActions[action] {
			return "confirm"
		}
		return "escalate"
	}
	if sensitiveActions[action] || alwaysShown[action] {
		return "confirm"
	}
	if p >= t.Auto {
		return "auto"
	}
	return "confirm"
}
