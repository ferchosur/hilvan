import type { Destination } from '../domain/index.js';

/** A command as an adapter describes it, independent of where it runs. */
export interface Command {
  executable: string;
  args: readonly string[];
  /** Variables to set for this command only (e.g. CLAUDE_CONFIG_DIR). */
  env: Readonly<Record<string, string>>;
  /** Variables to remove from the inherited environment (e.g. a stray API key). */
  unsetEnv?: readonly string[];
  /** Working directory inside the destination environment. */
  cwd?: string;
}

/** The concrete process to spawn on the host. Always spawned without a shell. */
export interface Invocation {
  command: string;
  args: string[];
  env: NodeJS.ProcessEnv;
  cwd?: string;
}

export class LauncherError extends Error {
  override name = 'LauncherError';
}

export interface HostContext {
  platform: NodeJS.Platform;
  /** True when the host is Linux running inside WSL. */
  insideWsl: boolean;
  env: NodeJS.ProcessEnv;
}

function hostEnv(host: HostContext, cmd: Command): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = { ...host.env };
  for (const k of cmd.unsetEnv ?? []) delete env[k];
  return env;
}

/**
 * Turns a command into a host invocation for the profile's destination (ADR-0005).
 * Pure function: no I/O, so every launcher is unit-tested on any OS.
 */
export function buildInvocation(dest: Destination, cmd: Command, host: HostContext): Invocation {
  const executable = dest.executable ?? cmd.executable;
  switch (dest.launcher) {
    case 'native': {
      const env = { ...hostEnv(host, cmd), ...cmd.env };
      return {
        command: executable,
        args: [...cmd.args],
        env,
        ...(cmd.cwd ? { cwd: cmd.cwd } : {}),
      };
    }
    case 'wsl': {
      if (host.platform !== 'win32') {
        throw new LauncherError('the wsl launcher is only available when Hilvan runs on Windows');
      }
      if (!dest.distro) throw new LauncherError('the wsl launcher requires a distro');
      // Variables are set inside the distribution with `env`, so they never leak between profiles.
      const unset = (cmd.unsetEnv ?? []).flatMap((k) => ['-u', k]);
      const assignments = Object.entries(cmd.env).map(([k, v]) => `${k}=${v}`);
      const cd = cmd.cwd ? ['--cd', cmd.cwd] : [];
      return {
        command: 'wsl.exe',
        args: [
          '-d',
          dest.distro,
          ...cd,
          '--',
          'env',
          ...unset,
          ...assignments,
          executable,
          ...cmd.args,
        ],
        env: hostEnv(host, cmd),
      };
    }
    case 'windows': {
      if (!host.insideWsl) {
        throw new LauncherError(
          'the windows launcher is only available when Hilvan runs inside WSL',
        );
      }
      // WSL interop forwards variables listed in WSLENV to Windows processes.
      const base = hostEnv(host, cmd);
      const names = Object.keys(cmd.env);
      const wslenv = [base.WSLENV, ...names].filter(Boolean).join(':');
      return {
        command: executable,
        args: [...cmd.args],
        env: { ...base, ...cmd.env, WSLENV: wslenv },
        ...(cmd.cwd ? { cwd: cmd.cwd } : {}),
      };
    }
  }
}
