package policy

import (
	"strings"
	"testing"

	"github.com/ferchosur/hilvan/internal/domain"
)

func prof(id, app string, accepts, tags, provides []string) *domain.Profile {
	cfg := "/h/" + id
	p := &domain.Profile{
		ID: id, Label: id, App: app, Auth: domain.Auth{Mode: "subscription"}, ConfigDir: &cfg,
		Destination: domain.Destination{Launcher: "native"}, Accepts: accepts, Tags: tags,
		Tools: domain.Tools{Provides: provides},
	}
	p.Normalize()
	return p
}

var (
	corp     = prof("claude-corp", "claude-code", []string{"public", "corporate", "confidential"}, []string{"corporate"}, []string{"jira"})
	personal = prof("claude-personal", "claude-code", []string{"public", "personal", "corporate"}, []string{"personal"}, nil)

	approvalRule = domain.Rule{
		ID:     "corp-to-personal",
		From:   domain.Selector{Classification: []string{"corporate", "confidential"}},
		To:     domain.Selector{Tags: []string{"personal"}},
		Effect: "require_approval",
		Reason: "corporate data to a personal account needs approval",
	}
	corpToPersonal = &domain.PolicyFile{Version: 1, Rules: []domain.Rule{approvalRule}}
)

func corporate(requires ...string) TaskFacts {
	return TaskFacts{Classification: "corporate", Requires: requires}
}

func TestToolOnlyOnCorporateProfile(t *testing.T) {
	p := Merge(Layers{})
	if !Evaluate(p, corporate("jira"), corp, nil).Eligible {
		t.Error("corporate profile with jira must be eligible")
	}
	ev := Evaluate(p, corporate("jira"), personal, nil)
	if ev.Eligible || !strings.Contains(ev.Exclusions[0].Reason, "lacks required tools: jira") {
		t.Errorf("AC-006: %+v", ev)
	}
}

func TestClassificationExcludes(t *testing.T) {
	ev := Evaluate(Merge(Layers{}), TaskFacts{Classification: "confidential"}, personal, nil)
	if ev.Eligible || !strings.Contains(ev.Exclusions[0].Reason, `does not accept "confidential"`) {
		t.Errorf("AC-012: %+v", ev)
	}
}

func TestRequireApproval(t *testing.T) {
	pol := Merge(Layers{User: corpToPersonal})
	if ev := Evaluate(pol, corporate(), personal, nil); !ev.Eligible || !ev.RequiresApproval {
		t.Errorf("FR-004: %+v", ev)
	}
	if Evaluate(pol, corporate(), corp, nil).RequiresApproval {
		t.Error("the corporate profile needs no approval")
	}
}

func TestEnforcedOnlyInManagedLayer(t *testing.T) {
	deny := approvalRule
	deny.ID, deny.Effect, deny.Enforced = "no-corp-out", "deny", true
	managed := Merge(Layers{Managed: &domain.PolicyFile{Version: 1, Rules: []domain.Rule{deny}}})
	ev := Evaluate(managed, corporate(), personal, nil)
	if ev.Eligible || !ev.Exclusions[len(ev.Exclusions)-1].Enforced {
		t.Errorf("managed enforced deny: %+v", ev)
	}
	user := Merge(Layers{User: &domain.PolicyFile{Version: 1, Rules: []domain.Rule{deny}}})
	if user.Rules[0].Enforced || !strings.Contains(strings.Join(user.Warnings, ","), "only the managed policy can enforce") {
		t.Errorf("SR-020: %+v", user)
	}
}

func TestProjectCannotRedefineClassifications(t *testing.T) {
	p := Merge(Layers{Project: &domain.PolicyFile{Version: 1, Classifications: []string{"public"}}})
	if !contains(p.Classifications, "confidential") || !strings.Contains(strings.Join(p.Warnings, ","), "cannot redefine") {
		t.Errorf("SR-019: %+v", p)
	}
}

func TestHandoffSelectorUsesSourceProfile(t *testing.T) {
	rule := domain.Rule{ID: "from-corp", From: domain.Selector{Tags: []string{"corporate"}}, To: domain.Selector{Apps: []string{"claude-code"}}, Effect: "deny", Reason: "no"}
	pol := Merge(Layers{User: &domain.PolicyFile{Version: 1, Rules: []domain.Rule{rule}}})
	if Evaluate(pol, TaskFacts{Classification: "public"}, personal, nil).Eligible == false {
		t.Error("without a source profile, a from.tags selector does not match")
	}
	if Evaluate(pol, TaskFacts{Classification: "public"}, personal, corp).Eligible {
		t.Error("hand-off from the corporate profile must be denied")
	}
}
