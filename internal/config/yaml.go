package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"

	"hilvan/internal/domain"
	"hilvan/internal/security"
)

// MaxConfigBytes bounds configuration files (SR-008).
const MaxConfigBytes = 1024 * 1024

// maxAliasCount matches the TypeScript limit (yaml's maxAliasCount: 100).
const maxAliasCount = 100

// ConfigError is an invalid or unsafe configuration file.
type ConfigError struct {
	File    string
	Details string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("Invalid configuration in %s:\n%s", e.File, e.Details)
}

func countAliases(n *yaml.Node) int {
	if n == nil {
		return 0
	}
	c := 0
	if n.Kind == yaml.AliasNode {
		c++
	}
	for _, child := range n.Content {
		c += countAliases(child)
	}
	return c
}

// decodeYAML parses one YAML document into v with the SR-008 limits: duplicate keys are errors
// (yaml.v3 default) and at most maxAliasCount aliases. strict rejects unknown fields.
// empty is true for an empty or null document.
func decodeYAML(text string, v any, strict bool) (empty bool, err error) {
	if len(text) > MaxConfigBytes {
		return false, fmt.Errorf("file exceeds %d bytes", MaxConfigBytes)
	}
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(text), &root); err != nil {
		return false, err
	}
	if countAliases(&root) > maxAliasCount {
		return false, errors.New("excessive alias count indicates a resource exhaustion attack")
	}
	if len(root.Content) == 0 || (root.Content[0].Kind == yaml.ScalarNode && root.Content[0].Tag == "!!null") {
		return true, nil
	}
	dec := yaml.NewDecoder(bytes.NewReader([]byte(text)))
	dec.KnownFields(strict)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return false, nil
}

// ParseProfiles parses and validates the text of a profiles.yaml file.
func ParseProfiles(text, file string) (domain.ProfilesFile, error) {
	var f domain.ProfilesFile
	empty, err := decodeYAML(text, &f, false)
	if err != nil {
		return f, &ConfigError{File: file, Details: err.Error()}
	}
	if empty {
		f = domain.ProfilesFile{Version: 1}
	}
	f.Normalize()
	if issues := f.Validate(); len(issues) > 0 {
		return f, &ConfigError{File: file, Details: issues.Pretty()}
	}
	return f, nil
}

// LoadProfiles loads profiles.yaml; a missing file means no profiles yet.
func LoadProfiles(file string) (domain.ProfilesFile, error) {
	text, found, err := security.ReadTextSafe(file, MaxConfigBytes)
	if err != nil {
		if security.IsUnsafePath(err) {
			return domain.ProfilesFile{}, &ConfigError{File: file, Details: err.Error()}
		}
		return domain.ProfilesFile{}, err
	}
	if !found {
		return domain.ProfilesFile{Version: 1, Profiles: []domain.Profile{}}, nil
	}
	return ParseProfiles(text, file)
}

const profilesHeader = "# Hilvan profiles — managed with `hilvan profile`. Never put secrets here.\n"

// SaveProfiles writes profiles.yaml atomically; the config directory is private to the user.
func SaveProfiles(file string, data domain.ProfilesFile) error {
	data.Normalize()
	if issues := data.Validate(); len(issues) > 0 {
		return &ConfigError{File: file, Details: issues.Pretty()}
	}
	var b bytes.Buffer
	b.WriteString(profilesHeader)
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(data); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return security.WriteFileAtomic(file, b.Bytes(), 0o600)
}
