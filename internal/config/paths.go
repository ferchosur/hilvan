// Package config locates and loads Hilvan's user configuration (ADR-0007): profiles, the
// layered policy and Criterio's learned model.
package config

import (
	"os"
	"path"
	"runtime"
	"strings"
)

// AppDirName is the name of Hilvan's configuration directory.
const AppDirName = "hilvan"

// Platform describes the host so paths for any OS can be computed (and tested) anywhere.
type Platform struct {
	// OS uses Node's names: linux, win32, darwin.
	OS      string
	Env     map[string]string
	Homedir string
}

// CurrentPlatform describes this process's host.
func CurrentPlatform() Platform {
	osName := runtime.GOOS
	if osName == "windows" {
		osName = "win32"
	}
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && k != "" {
			env[k] = v
		}
	}
	home, _ := os.UserHomeDir()
	return Platform{OS: osName, Env: env, Homedir: home}
}

// Join joins path elements with the separator and normalisation of p's OS.
func (p Platform) Join(elem ...string) string {
	if p.OS != "win32" {
		return path.Join(elem...)
	}
	parts := []string{}
	for _, e := range elem {
		if e != "" {
			parts = append(parts, e)
		}
	}
	if len(parts) == 0 {
		return "."
	}
	joined := strings.ReplaceAll(strings.Join(parts, `\`), "/", `\`)
	unc := strings.HasPrefix(joined, `\\`)
	for strings.Contains(joined, `\\`) {
		joined = strings.ReplaceAll(joined, `\\`, `\`)
	}
	if unc {
		joined = `\` + joined
	}
	return joined
}

// UserConfigDir is the per-user configuration directory. HILVAN_CONFIG_DIR overrides it.
func (p Platform) UserConfigDir() string {
	if v := p.Env["HILVAN_CONFIG_DIR"]; v != "" {
		return v
	}
	switch p.OS {
	case "win32":
		appData, ok := p.Env["APPDATA"]
		if !ok {
			appData = p.Join(p.Homedir, "AppData", "Roaming")
		}
		return p.Join(appData, AppDirName)
	case "darwin":
		return p.Join(p.Homedir, "Library", "Application Support", AppDirName)
	}
	xdg := p.Env["XDG_CONFIG_HOME"]
	if xdg == "" {
		xdg = p.Join(p.Homedir, ".config")
	}
	return p.Join(xdg, AppDirName)
}

// ManagedPolicyFile is the read-only policy whose enforced rules users cannot approve over.
func (p Platform) ManagedPolicyFile() string {
	switch p.OS {
	case "win32":
		programData, ok := p.Env["ProgramData"]
		if !ok {
			programData = `C:\ProgramData`
		}
		return p.Join(programData, AppDirName, "policy.yaml")
	case "darwin":
		return p.Join("/Library/Application Support", AppDirName, "policy.yaml")
	}
	return p.Join("/etc", AppDirName, "policy.yaml")
}

// ProfilesFile is profiles.yaml in the user configuration directory.
func (p Platform) ProfilesFile() string { return p.Join(p.UserConfigDir(), "profiles.yaml") }

// UserPolicyFile is the user's policy.yaml.
func (p Platform) UserPolicyFile() string { return p.Join(p.UserConfigDir(), "policy.yaml") }
