package spike006

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestYAMLWrittenByTS(t *testing.T) {
	y, _ := os.ReadFile("testdata/profiles.yaml")
	j, _ := os.ReadFile("testdata/profiles.json")
	var fromYAML, fromJSON any
	if err := yaml.Unmarshal(y, &fromYAML); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(j, &fromJSON)
	// Normalise ints to float64 as JSON does.
	b, _ := json.Marshal(fromYAML)
	var norm any
	json.Unmarshal(b, &norm)
	if !reflect.DeepEqual(norm, fromJSON) {
		t.Fatalf("yaml:\n%s\n\nyaml->json: %s\njson:       %s", y, b, j)
	}
}

func TestYAMLTypedStrings(t *testing.T) {
	y, _ := os.ReadFile("testdata/profiles.yaml")
	var doc struct {
		Profiles []struct {
			Tags      []string `yaml:"tags"`
			ConfigDir *string  `yaml:"configDir"`
		} `yaml:"profiles"`
	}
	if err := yaml.Unmarshal(y, &doc); err != nil {
		t.Fatal(err)
	}
	want := []string{"yes", "no", "on", "off", "null", "~", "0700", "1e3", "2026-10-07"}
	if got := doc.Profiles[0].Tags; !reflect.DeepEqual(got, want) || doc.Profiles[0].ConfigDir != nil {
		t.Fatalf("tags %q configDir %v", got, doc.Profiles[0].ConfigDir)
	}
}
