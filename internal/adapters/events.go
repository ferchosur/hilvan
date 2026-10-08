// Package adapters drives each agent CLI as a non-interactive process (ADR-0004,
// docs/design/contrato-adaptador.md): commands, identity checks, JSONL parsing and the run's
// final state.
package adapters

import (
	"regexp"

	"github.com/ferchosur/hilvan/internal/domain"
)

// Event types.
const (
	EvSession = "session"
	EvMessage = "message"
	EvTool    = "tool"
	EvRetry   = "retry"
	EvLimits  = "limits"
	EvUsage   = "usage"
	EvResult  = "result"
	EvRaw     = "raw"
)

// Event is a normalised agent event. Only the fields of its Type are meaningful; pointer
// fields distinguish "absent" from zero, as in the TypeScript union.
type Event struct {
	Type string

	SessionID  *string  // session, result
	Model      string   // session
	MCPServers []string // session (Claude reports the list, possibly empty; Codex does not)

	Text string // message, result

	Name    string // tool
	Status  string // tool: started|completed|failed; limits: allowed|…
	Summary string // tool

	Attempt int    // retry
	Error   string // retry; result
	// HasError marks a result that carries an error field (even an empty one), like `error` in TS.
	HasError bool

	Windows []domain.LimitWindow // limits

	CostUSD      *float64 // usage
	InputTokens  *float64 // usage
	OutputTokens *float64 // usage

	OK                bool   // result
	StopReason        string // result
	PermissionDenials int    // result

	Line string // raw
}

// Map renders the event like the TypeScript object, for logs and parity tests.
func (e Event) Map() map[string]any {
	m := map[string]any{"type": e.Type}
	switch e.Type {
	case EvSession:
		m["sessionId"] = deref(e.SessionID)
		if e.Model != "" {
			m["model"] = e.Model
		}
		if e.MCPServers != nil {
			m["mcpServers"] = e.MCPServers
		}
	case EvMessage:
		m["text"] = e.Text
	case EvTool:
		m["name"], m["status"] = e.Name, e.Status
		if e.Summary != "" {
			m["summary"] = e.Summary
		}
	case EvRetry:
		m["attempt"], m["error"] = e.Attempt, e.Error
	case EvLimits:
		m["status"] = e.Status
		m["windows"] = windowsMaps(e.Windows)
	case EvUsage:
		setNum(m, "costUsd", e.CostUSD)
		setNum(m, "inputTokens", e.InputTokens)
		setNum(m, "outputTokens", e.OutputTokens)
	case EvResult:
		m["ok"], m["text"] = e.OK, e.Text
		if e.SessionID != nil {
			m["sessionId"] = *e.SessionID
		}
		if e.StopReason != "" {
			m["stopReason"] = e.StopReason
		}
		if e.PermissionDenials != 0 {
			m["permissionDenials"] = e.PermissionDenials
		}
		if e.HasError {
			m["error"] = e.Error
		}
	case EvRaw:
		m["line"] = e.Line
	}
	return m
}

func windowsMaps(ws []domain.LimitWindow) []any {
	out := []any{}
	for _, w := range ws {
		m := map[string]any{"name": w.Name, "utilization": w.Utilization}
		if w.ResetsAt != nil {
			m["resetsAt"] = *w.ResetsAt
		}
		out = append(out, m)
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func setNum(m map[string]any, k string, v *float64) {
	if v != nil {
		m[k] = *v
	}
}

// Outcome is the final state of a run.
type Outcome struct {
	State             string // completed|failed|blocked|cancelled|uncertain
	BlockedReason     string
	SessionID         *string
	ResultText        *string
	PermissionDenials int
	CostUSD           *float64
	InputTokens       *float64
	OutputTokens      *float64
	Limits            []domain.LimitWindow // nil = never reported
	Error             *string
}

// Map renders the outcome like the TypeScript object, for parity tests.
func (o Outcome) Map() map[string]any {
	m := map[string]any{"state": o.State}
	if o.BlockedReason != "" {
		m["blockedReason"] = o.BlockedReason
	}
	if o.SessionID != nil {
		m["sessionId"] = *o.SessionID
	}
	if o.ResultText != nil {
		m["resultText"] = *o.ResultText
	}
	if o.PermissionDenials != 0 {
		m["permissionDenials"] = o.PermissionDenials
	}
	setNum(m, "costUsd", o.CostUSD)
	setNum(m, "inputTokens", o.InputTokens)
	setNum(m, "outputTokens", o.OutputTokens)
	if o.Limits != nil {
		m["limits"] = windowsMaps(o.Limits)
	}
	if o.Error != nil {
		m["error"] = *o.Error
	}
	return m
}

var (
	rateLimitText = regexp.MustCompile(`(?i)rate.?limit|usage limit|quota`)
	authText      = regexp.MustCompile(`(?i)auth|login|unauthori[sz]ed|401|403`)
	billingText   = regexp.MustCompile(`(?i)billing|credit`)
	authKinds     = map[string]bool{"authentication_failed": true, "oauth_org_not_allowed": true, "account_on_hold": true}
)

func classifyError(err string) string {
	switch {
	case err == "rate_limit" || rateLimitText.MatchString(err):
		return "rate_limit"
	case authKinds[err] || authText.MatchString(err):
		return "auth"
	case err == "billing_error" || billingText.MatchString(err):
		return "billing"
	}
	return ""
}

func add(acc **float64, v *float64) {
	if v == nil {
		return
	}
	sum := *v
	if *acc != nil {
		sum += **acc
	}
	*acc = &sum
}

func str(s string) *string { return &s }

// DecideOutcome derives the final state from the events, never from the exit code alone
// (spike 002: Claude exits 0 after SIGINT). FR-017: limits/auth/billing → blocked; no final
// event → uncertain.
func DecideOutcome(events []Event, cancelRequested bool) Outcome {
	out := Outcome{State: domain.RunUncertain}
	var lastRetry *string
	var result *Event
	for i := range events {
		e := events[i]
		switch e.Type {
		case EvSession:
			out.SessionID = str(deref(e.SessionID))
		case EvRetry:
			lastRetry = str(e.Error)
		case EvLimits:
			out.Limits = append([]domain.LimitWindow{}, e.Windows...)
			if e.Status != "allowed" && e.Status != "allowed_warning" && lastRetry == nil {
				lastRetry = str("rate_limit")
			}
		case EvUsage:
			add(&out.CostUSD, e.CostUSD)
			add(&out.InputTokens, e.InputTokens)
			add(&out.OutputTokens, e.OutputTokens)
		case EvResult:
			result = &events[i]
		}
	}
	if result != nil && deref(result.SessionID) != "" {
		out.SessionID = result.SessionID
	}
	if result != nil && result.PermissionDenials != 0 {
		out.PermissionDenials = result.PermissionDenials
	}
	if result == nil {
		if cancelRequested {
			out.State = domain.RunCancelled
			return out
		}
		if lastRetry != nil {
			if reason := classifyError(*lastRetry); reason != "" {
				out.State, out.BlockedReason = domain.RunBlocked, reason
			}
		}
		return out
	}
	out.ResultText = str(result.Text)
	if result.OK {
		out.State = domain.RunCompleted
		return out
	}
	if cancelRequested {
		out.State = domain.RunCancelled
		return out
	}
	// Classify from structured error fields only; free result text may mention "auth" innocently.
	structured, errText := "", result.Text
	switch {
	case result.HasError:
		structured, errText = result.Error, result.Error
	case lastRetry != nil:
		structured = *lastRetry
	}
	if reason := classifyError(structured); reason != "" {
		out.State, out.BlockedReason, out.Error = domain.RunBlocked, reason, str(errText)
		return out
	}
	out.State, out.Error = domain.RunFailed, str(errText)
	return out
}
