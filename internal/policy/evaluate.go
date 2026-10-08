// Package policy merges policy layers and decides profile eligibility (ADR-0009, FR-011).
package policy

import (
	"strings"

	"hilvan/internal/domain"
)

// Layers holds the policy files that exist; nil means the layer is absent.
type Layers struct {
	Managed, User, Project *domain.PolicyFile
}

// Merge combines the layers:
//   - classifications come from the managed (or user) file; projects cannot redefine them;
//   - enforced is honoured only in the managed layer (SR-020);
//   - project rules can only restrict: there is no allow effect (SR-019).
func Merge(l Layers) domain.EffectivePolicy {
	warnings := []string{}
	classifications := append([]string{}, domain.DefaultClassifications...)
	switch {
	case l.Managed != nil && l.Managed.Classifications != nil:
		classifications = l.Managed.Classifications
	case l.User != nil && l.User.Classifications != nil:
		classifications = l.User.Classifications
	}
	if l.Project != nil && l.Project.Classifications != nil {
		warnings = append(warnings, "project policy cannot redefine classifications; ignored")
	}
	rules := []domain.LayeredRule{}
	for _, layer := range []struct {
		name string
		file *domain.PolicyFile
	}{{domain.LayerManaged, l.Managed}, {domain.LayerUser, l.User}, {domain.LayerProject, l.Project}} {
		if layer.file == nil {
			continue
		}
		for _, r := range layer.file.Rules {
			if r.Enforced && layer.name != domain.LayerManaged {
				warnings = append(warnings, "rule "+r.ID+" ("+layer.name+") is marked enforced; only the managed policy can enforce")
			}
			r.Enforced = layer.name == domain.LayerManaged && r.Enforced
			rules = append(rules, domain.LayeredRule{Rule: r, Layer: layer.name})
		}
	}
	return domain.EffectivePolicy{Classifications: classifications, Rules: rules, Warnings: warnings}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func matches(sel domain.Selector, classification string, p *domain.Profile) bool {
	if sel.Classification != nil && !(classification != "" && contains(sel.Classification, classification)) {
		return false
	}
	if sel.Profiles != nil && !(p != nil && contains(sel.Profiles, p.ID)) {
		return false
	}
	if sel.Tags != nil {
		ok := false
		if p != nil {
			for _, t := range sel.Tags {
				ok = ok || contains(p.Tags, t)
			}
		}
		if !ok {
			return false
		}
	}
	if sel.Apps != nil && !(p != nil && contains(sel.Apps, p.App)) {
		return false
	}
	return true
}

// Exclusion is why a profile cannot receive a task.
type Exclusion struct {
	Reason string
	// Source is classification, tools, routing or rule:<id>.
	Source string
	// Enforced exclusions cannot be approved over (AC-014).
	Enforced bool
}

// Approval is a rule that requires the user's explicit approval for a transfer.
type Approval struct {
	RuleID string
	Reason string
	Layer  domain.PolicyLayer
}

// Evaluation is the eligibility of one profile for one task.
type Evaluation struct {
	ProfileID string
	Eligible  bool
	// RequiresApproval: eligible, but a rule demands explicit approval for this transfer.
	RequiresApproval bool
	Exclusions       []Exclusion
	Approvals        []Approval
}

// TaskFacts are the task fields that eligibility depends on.
type TaskFacts struct {
	Classification string
	Requires       []string
}

// Evaluate checks eligibility before any optimisation (D-008, FR-011): classification accepted,
// required tools present, no deny rule. from is set for hand-offs (data leaves that profile).
func Evaluate(pol domain.EffectivePolicy, task TaskFacts, profile *domain.Profile, from *domain.Profile) Evaluation {
	exclusions := []Exclusion{}
	approvals := []Approval{}
	if !contains(pol.Classifications, task.Classification) {
		exclusions = append(exclusions, Exclusion{Reason: `unknown classification "` + task.Classification + `"`, Source: "classification", Enforced: true})
	}
	if !contains(profile.Accepts, task.Classification) {
		exclusions = append(exclusions, Exclusion{Reason: `does not accept "` + task.Classification + `" data`, Source: "classification"})
	}
	var missing []string
	for _, r := range task.Requires {
		if !contains(profile.Tools.Provides, r) {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		exclusions = append(exclusions, Exclusion{Reason: "lacks required tools: " + strings.Join(missing, ", "), Source: "tools"})
	}
	for _, r := range pol.Rules {
		if !matches(r.From, task.Classification, from) || !matches(r.To, "", profile) {
			continue
		}
		if r.Effect == "deny" {
			exclusions = append(exclusions, Exclusion{Reason: r.Reason, Source: "rule:" + r.ID, Enforced: r.Enforced})
		} else {
			approvals = append(approvals, Approval{RuleID: r.ID, Reason: r.Reason, Layer: r.Layer})
		}
	}
	eligible := len(exclusions) == 0
	return Evaluation{
		ProfileID:        profile.ID,
		Eligible:         eligible,
		RequiresApproval: eligible && len(approvals) > 0,
		Exclusions:       exclusions,
		Approvals:        approvals,
	}
}
