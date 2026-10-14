package domain

import "testing"

// SR-038: web tools are allowed by default for public and personal tasks; the profile can turn
// them off or change the classifications that may use them.
func TestWebFor(t *testing.T) {
	p := &Profile{ID: "p"}
	if !p.WebFor("public") || !p.WebFor("personal") || p.WebFor("corporate") || p.WebFor("confidential") {
		t.Errorf("default web rule: public=%v personal=%v corporate=%v", p.WebFor("public"), p.WebFor("personal"), p.WebFor("corporate"))
	}
	off := false
	p.Tools.Web = &off
	if p.WebFor("public") {
		t.Error("tools.web: false must disable web")
	}
	on := true
	p.Tools.Web = &on
	p.Tools.WebClassifications = []string{"corporate"}
	if !p.WebFor("corporate") || p.WebFor("public") {
		t.Error("custom webClassifications not applied")
	}
}
