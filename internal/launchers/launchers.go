// Package launchers turns an adapter's command into the concrete host process for a profile's
// destination (ADR-0005): native, wsl (from Windows into a distro) or windows (from WSL).
// Everything here is pure, so every launcher is unit-tested on any OS.
package launchers

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ferchosur/hilvan/internal/domain"
)

// Command is what an adapter wants to run, independent of where it runs.
type Command struct {
	Executable string
	Args       []string
	// Env holds variables to set for this command only (e.g. CLAUDE_CONFIG_DIR).
	Env map[string]string
	// UnsetEnv lists variables to remove from the inherited environment (e.g. a stray API key).
	UnsetEnv []string
	// Cwd is the working directory as the destination sees it (see DestinationPath).
	Cwd string
}

// Invocation is the process to spawn on the host, always without a shell.
type Invocation struct {
	Command string
	Args    []string
	// Env is the complete environment of the process.
	Env map[string]string
	// Cwd is the working directory as the host sees it ("" = inherit).
	Cwd string
}

// Host describes where Hilvan runs.
type Host struct {
	// OS uses Node's names: linux, win32, darwin.
	OS string
	// InsideWSL is true when the host is Linux running inside WSL.
	InsideWSL bool
	Env       map[string]string
}

// ErrLauncher reports a destination that cannot be reached from this host.
var ErrLauncher = errors.New("launcher")

func launcherError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrLauncher, fmt.Sprintf(format, args...))
}

func sameName(h Host, a, b string) bool {
	if h.OS == "win32" {
		return strings.EqualFold(a, b) // Windows variable names are case-insensitive
	}
	return a == b
}

func hostEnv(h Host, cmd Command) map[string]string {
	env := map[string]string{}
	for k, v := range h.Env {
		removed := false
		for _, u := range cmd.UnsetEnv {
			removed = removed || sameName(h, k, u)
		}
		if !removed {
			env[k] = v
		}
	}
	return env
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// BuildInvocation turns a command into a host invocation for the profile's destination.
func BuildInvocation(dest domain.Destination, cmd Command, h Host) (Invocation, error) {
	executable := cmd.Executable
	if dest.Executable != "" {
		executable = dest.Executable
	}
	switch dest.Launcher {
	case domain.LauncherNative:
		env := hostEnv(h, cmd)
		for k, v := range cmd.Env {
			env[k] = v
		}
		return Invocation{Command: executable, Args: append([]string{}, cmd.Args...), Env: env, Cwd: cmd.Cwd}, nil

	case domain.LauncherWSL:
		if h.OS != "win32" {
			return Invocation{}, launcherError("the wsl launcher is only available when Hilvan runs on Windows")
		}
		if dest.Distro == "" {
			return Invocation{}, launcherError("the wsl launcher requires a distro")
		}
		// Variables are set inside the distribution with env, so they never leak between
		// profiles. wsl.exe does not load the user's shell profile: PATH comes from the profile.
		args := []string{"-d", dest.Distro}
		if cmd.Cwd != "" {
			args = append(args, "--cd", cmd.Cwd)
		}
		args = append(args, "--", "env")
		for _, k := range cmd.UnsetEnv {
			args = append(args, "-u", k)
		}
		if dest.Path != "" {
			args = append(args, "PATH="+dest.Path)
		}
		for _, k := range sortedKeys(cmd.Env) {
			args = append(args, k+"="+cmd.Env[k])
		}
		args = append(args, executable)
		args = append(args, cmd.Args...)
		return Invocation{Command: "wsl.exe", Args: args, Env: hostEnv(h, cmd)}, nil

	case domain.LauncherWindows:
		if !h.InsideWSL {
			return Invocation{}, launcherError("the windows launcher is only available when Hilvan runs inside WSL")
		}
		// WSL interop forwards the variables listed in WSLENV to Windows processes. Profile values
		// are already Windows paths, so they are forwarded as they are.
		env := hostEnv(h, cmd)
		names := []string{}
		if w := env["WSLENV"]; w != "" {
			names = append(names, w)
		}
		for _, k := range sortedKeys(cmd.Env) {
			env[k] = cmd.Env[k]
			names = append(names, k)
		}
		env["WSLENV"] = strings.Join(names, ":")
		cwd := ""
		if cmd.Cwd != "" {
			p, err := WindowsToWSL(cmd.Cwd, h.Env["WSL_DISTRO_NAME"])
			if err != nil {
				return Invocation{}, err
			}
			cwd = p
		}
		return Invocation{Command: executable, Args: append([]string{}, cmd.Args...), Env: env, Cwd: cwd}, nil
	}
	return Invocation{}, launcherError("unknown launcher %q", dest.Launcher)
}

// DestinationPath converts a host path (e.g. a worktree) into the path the destination's agent
// must use. The same repository on the Windows disk is C:\… on Windows and /mnt/c/… in WSL.
func DestinationPath(dest domain.Destination, h Host, hostPath string) (string, error) {
	switch dest.Launcher {
	case domain.LauncherWSL:
		return WindowsToWSL(hostPath, dest.Distro)
	case domain.LauncherWindows:
		return WSLToWindows(hostPath, h.Env["WSL_DISTRO_NAME"])
	}
	return hostPath, nil
}

func isDriveLetter(c byte) bool { return (c|0x20) >= 'a' && (c|0x20) <= 'z' }

// WindowsToWSL maps C:\a\b to /mnt/c/a/b and \\wsl.localhost\<distro>\x (or \\wsl$\…) to /x.
// It assumes WSL's default automount root (/mnt). A path that is already a Linux path is
// returned unchanged.
func WindowsToWSL(p, distro string) (string, error) {
	if strings.HasPrefix(p, "/") {
		return p, nil
	}
	s := strings.ReplaceAll(p, `\`, "/")
	if len(s) >= 2 && isDriveLetter(s[0]) && s[1] == ':' {
		rest := strings.TrimPrefix(s[2:], "/")
		out := "/mnt/" + strings.ToLower(s[:1])
		if rest != "" {
			out += "/" + rest
		}
		return strings.TrimSuffix(out, "/"), nil
	}
	for _, prefix := range []string{"//wsl.localhost/", "//wsl$/"} {
		if len(s) > len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			name, rest, _ := strings.Cut(s[len(prefix):], "/")
			if distro != "" && !strings.EqualFold(name, distro) {
				return "", launcherError("%s belongs to distro %q, not %q", p, name, distro)
			}
			return "/" + rest, nil
		}
	}
	return "", launcherError("cannot map %s into WSL", p)
}

// WSLToWindows maps /mnt/c/a/b to C:\a\b and any other absolute Linux path to
// \\wsl.localhost\<distro>\path (distro from WSL_DISTRO_NAME).
func WSLToWindows(p, distro string) (string, error) {
	if !strings.HasPrefix(p, "/") {
		return p, nil // already a Windows path
	}
	if strings.HasPrefix(p, "/mnt/") && len(p) >= 6 && isDriveLetter(p[5]) && (len(p) == 6 || p[6] == '/') {
		rest := strings.ReplaceAll(strings.TrimPrefix(p[6:], "/"), "/", `\`)
		return strings.ToUpper(p[5:6]) + `:\` + rest, nil
	}
	if distro == "" {
		return "", launcherError("cannot map %s to Windows: WSL_DISTRO_NAME is not set", p)
	}
	return `\\wsl.localhost\` + distro + strings.ReplaceAll(p, "/", `\`), nil
}
