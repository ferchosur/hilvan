package config

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"hilvan/internal/criterio"
	"hilvan/internal/security"
)

// MaxModelBytes bounds Criterio's model file.
const MaxModelBytes = 8 * 1024 * 1024

// CriterioFile is the learned model inside a configuration directory.
func CriterioFile(configDir string) string {
	return filepath.Join(configDir, "criterio", "model.json")
}

// LoadCriterio loads the learned model. A missing, oversized or invalid file yields a fresh
// engine (rules only) and, except when missing, a warning.
func LoadCriterio(file string) (engine *criterio.Criterio, warning string, err error) {
	text, found, err := security.ReadTextSafe(file, MaxModelBytes)
	if err != nil {
		if security.IsUnsafePath(err) {
			reason := "is not a regular file"
			if strings.Contains(err.Error(), "exceeds") {
				reason = "is too large"
			}
			return criterio.New(), file + " " + reason + "; ignored", nil
		}
		return nil, "", err
	}
	if !found {
		return criterio.New(), "", nil
	}
	var s criterio.State
	if err := json.Unmarshal([]byte(text), &s); err != nil || s.Validate() != nil {
		return criterio.New(), file + " is invalid; using rules only", nil
	}
	return criterio.FromState(s), "", nil
}

// SaveCriterio writes the model atomically.
func SaveCriterio(file string, engine *criterio.Criterio) error {
	data, err := json.Marshal(engine.State())
	if err != nil {
		return err
	}
	return security.WriteFileAtomic(file, data, 0o600)
}
