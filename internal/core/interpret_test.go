package core

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The interpreter's answer becomes a command only when it is one of the closed operations (FR-022).
func TestValidatePlanRefusesWhatIsNotInTheList(t *testing.T) {
	for _, answer := range []string{
		`{"operation":"shell","title":"rm -rf"}`,
		`{"operation":"none"}`,
		`{"operation":""}`,
		`not json`,
		`{"operation":"task run","task":"-p"}`,
		`{"operation":"task new","title":"x","mode":"admin"}`,
	} {
		if _, err := ValidatePlan(answer); err == nil {
			t.Errorf("accepted %s", answer)
		}
	}
}

// A valid answer gives an argv with one element per value and no shell, as in the spec's example.
func TestPlanArgvForTaskNew(t *testing.T) {
	pl, err := ValidatePlan(`{"operation":"task new","title":"Lectura","objective":"Lee README.md y di su primera línea.","criteria":"Una línea","mode":"read"}`)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := pl.Argv()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"task", "new", "Lectura", "--mode", "read", "--objective", "Lee README.md y di su primera línea.", "--criteria", "Una línea"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("argv %q, want %q", argv, want)
	}
}

// Operations that need a task refuse to run without one, and task run keeps --profile optional.
func TestPlanArgvNeedsTaskForTaskOperations(t *testing.T) {
	pl, _ := ValidatePlan(`{"operation":"task run"}`)
	if _, err := pl.Argv(); err == nil {
		t.Error("task run without a task accepted")
	}
	pl, _ = ValidatePlan(`{"operation":"task run","task":"tsk_1"}`)
	if argv, _ := pl.Argv(); !reflect.DeepEqual(argv, []string{"task", "run", "tsk_1"}) {
		t.Errorf("argv %q", argv)
	}
}

// The schema lists exactly the closed operations plus none, and nothing else is allowed.
func TestInterpretSchemaMatchesTheList(t *testing.T) {
	var schema struct {
		Properties struct {
			Operation struct {
				Enum []string `json:"enum"`
			} `json:"operation"`
		} `json:"properties"`
		AdditionalProperties bool `json:"additionalProperties"`
	}
	if err := json.Unmarshal([]byte(InterpretSchema()), &schema); err != nil {
		t.Fatal(err)
	}
	if schema.AdditionalProperties {
		t.Error("the schema accepts extra properties")
	}
	got := strings.Join(schema.Properties.Operation.Enum, ",")
	for _, op := range append(append([]string{}, Operations...), "none") {
		if !strings.Contains(","+got+",", ","+op+",") {
			t.Errorf("schema lacks %q", op)
		}
	}
}
