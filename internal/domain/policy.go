package domain

// DefaultClassifications are used when no policy defines its own (D-022).
var DefaultClassifications = []string{"public", "personal", "corporate", "confidential"}

// Selector matches the data being sent or the profile receiving it. Empty fields match anything.
type Selector struct {
	Classification []string `json:"classification,omitempty" yaml:"classification,omitempty"`
	Profiles       []string `json:"profiles,omitempty" yaml:"profiles,omitempty"`
	Tags           []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Apps           []string `json:"apps,omitempty" yaml:"apps,omitempty"`
}

// Rule is a policy rule (ADR-0009). There is no allow effect: rules can only restrict (SR-019).
type Rule struct {
	ID string `json:"id" yaml:"id"`
	// From selects the data being sent: task classification and/or the source profile (hand-offs).
	From Selector `json:"from" yaml:"from"`
	// To selects the profile receiving the data.
	To     Selector `json:"to" yaml:"to"`
	Effect string   `json:"effect" yaml:"effect"`
	// Enforced is only honoured in the managed policy (SR-020).
	Enforced bool   `json:"enforced" yaml:"enforced"`
	Reason   string `json:"reason" yaml:"reason"`
}

// PolicyFile is the content of a policy.yaml layer.
type PolicyFile struct {
	Version         int      `json:"version" yaml:"version"`
	Classifications []string `json:"classifications,omitempty" yaml:"classifications,omitempty"`
	Rules           []Rule   `json:"rules" yaml:"rules"`
}

// PolicyLayer is managed, user or project.
type PolicyLayer = string

const (
	LayerManaged PolicyLayer = "managed"
	LayerUser    PolicyLayer = "user"
	LayerProject PolicyLayer = "project"
)

// LayeredRule is a rule with the layer it came from.
type LayeredRule struct {
	Rule
	Layer PolicyLayer `json:"layer"`
}

// EffectivePolicy is the merge of all layers.
type EffectivePolicy struct {
	Classifications []string
	Rules           []LayeredRule
	Warnings        []string
}

// Normalize fills defaults.
func (f *PolicyFile) Normalize() {
	if f.Rules == nil {
		f.Rules = []Rule{}
	}
}

func (s Selector) validate(b *issues, path string) {
	b.nonEmptyAll(field(path, "classification"), s.Classification)
	b.ids(field(path, "profiles"), s.Profiles)
	b.ids(field(path, "tags"), s.Tags)
	b.nonEmptyAll(field(path, "apps"), s.Apps)
}

// Validate checks a policy file. Unknown keys are rejected by the strict YAML decoder.
func (f *PolicyFile) Validate() Issues {
	var b issues
	if f.Version != 1 {
		b.add("version", "Invalid input: expected 1")
	}
	if f.Classifications != nil && len(f.Classifications) == 0 {
		b.add("classifications", "Too small: expected array to have >=1 items")
	}
	b.nonEmptyAll("classifications", f.Classifications)
	for i, r := range f.Rules {
		path := index("rules", i)
		b.id(field(path, "id"), r.ID)
		r.From.validate(&b, field(path, "from"))
		r.To.validate(&b, field(path, "to"))
		b.oneOf(field(path, "effect"), r.Effect, "deny", "require_approval")
		if n := utf16Len(r.Reason); n < 1 || n > 500 {
			b.add(field(path, "reason"), "reason must have 1 to 500 characters")
		}
	}
	return b.list
}
