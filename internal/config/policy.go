package config

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/policy"
	"github.com/ferchosur/hilvan/internal/security"
)

// LoadPolicyFile loads one policy layer. It returns nil when the file is missing or unreadable
// (an unreadable managed policy must not block the user). Unknown fields are rejected (SR-019).
func LoadPolicyFile(file string) (*domain.PolicyFile, error) {
	text, found, err := security.ReadTextSafe(file, MaxConfigBytes)
	if err != nil {
		if security.IsUnsafePath(err) {
			return nil, &ConfigError{File: file, Details: err.Error()}
		}
		if errors.Is(err, fs.ErrPermission) {
			return nil, nil
		}
		return nil, err
	}
	if !found {
		return nil, nil
	}
	var f domain.PolicyFile
	empty, err := decodeYAML(text, &f, true)
	if err != nil {
		return nil, &ConfigError{File: file, Details: err.Error()}
	}
	if empty {
		f = domain.PolicyFile{Version: 1}
	}
	f.Normalize()
	if issues := f.Validate(); len(issues) > 0 {
		return nil, &ConfigError{File: file, Details: issues.Pretty()}
	}
	return &f, nil
}

// PolicyPaths are the files of each layer; Project is empty outside a project.
type PolicyPaths struct {
	Managed, User, Project string
}

// DefaultPolicyPaths returns the layer files for this host and, optionally, a project root.
func DefaultPolicyPaths(p Platform, projectRoot string) PolicyPaths {
	paths := PolicyPaths{Managed: p.ManagedPolicyFile(), User: p.UserPolicyFile()}
	if projectRoot != "" {
		paths.Project = filepath.Join(projectRoot, ".hilvan", "policy.yaml")
	}
	return paths
}

// LoadEffectivePolicy loads and merges every layer.
func LoadEffectivePolicy(paths PolicyPaths) (domain.EffectivePolicy, error) {
	var layers policy.Layers
	var err error
	if layers.Managed, err = LoadPolicyFile(paths.Managed); err != nil {
		return domain.EffectivePolicy{}, err
	}
	if layers.User, err = LoadPolicyFile(paths.User); err != nil {
		return domain.EffectivePolicy{}, err
	}
	if paths.Project != "" {
		if layers.Project, err = LoadPolicyFile(paths.Project); err != nil {
			return domain.EffectivePolicy{}, err
		}
	}
	return policy.Merge(layers), nil
}
