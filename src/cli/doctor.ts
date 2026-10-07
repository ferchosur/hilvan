import { readFile } from 'node:fs/promises';
import { ConfigError, loadProfiles } from '../config/profiles.js';
import { type RunResult, runCommand } from '../process/run.js';

export type CheckStatus = 'ok' | 'warn' | 'fail' | 'info';

export interface CheckResult {
  name: string;
  status: CheckStatus;
  detail: string;
}

export interface DoctorDeps {
  platform: NodeJS.Platform;
  nodeVersion: string;
  profilesFile: string;
  run: (cmd: string, args: readonly string[]) => Promise<RunResult>;
  readText: (file: string) => Promise<string>;
}

export const MIN_NODE = [22, 12] as const;

export function defaultDoctorDeps(profilesFile: string): DoctorDeps {
  return {
    platform: process.platform,
    nodeVersion: process.versions.node,
    profilesFile,
    run: (cmd, args) => runCommand(cmd, args, { timeoutMs: 15_000 }),
    readText: (file) => readFile(file, 'utf8'),
  };
}

export function checkNode(version: string): CheckResult {
  const [major = 0, minor = 0] = version.split('.').map(Number);
  const ok = major > MIN_NODE[0] || (major === MIN_NODE[0] && minor >= MIN_NODE[1]);
  return {
    name: 'node',
    status: ok ? 'ok' : 'fail',
    detail: ok ? `v${version}` : `v${version} (requires >= ${MIN_NODE.join('.')})`,
  };
}

/** Checks an executable by asking for its version. `required` turns "missing" into a failure. */
export async function checkTool(
  deps: DoctorDeps,
  name: string,
  args: readonly string[],
  required: boolean,
): Promise<CheckResult> {
  const r = await deps.run(name, args);
  if (r.code === 0) {
    const firstLine = (r.stdout || r.stderr).trim().split(/\r?\n/)[0] ?? '';
    return { name, status: 'ok', detail: firstLine };
  }
  const reason = r.error === 'not_found' ? 'not found in PATH' : `exit ${r.code ?? r.error}`;
  return { name, status: required ? 'fail' : 'warn', detail: reason };
}

async function detectWsl(deps: DoctorDeps): Promise<CheckResult | null> {
  if (deps.platform === 'win32') {
    const r = await deps.run('wsl.exe', ['--version']);
    return r.code === 0
      ? { name: 'wsl', status: 'ok', detail: 'available (wsl launcher)' }
      : { name: 'wsl', status: 'info', detail: 'not available; wsl launcher disabled' };
  }
  if (deps.platform === 'linux') {
    try {
      const v = await deps.readText('/proc/version');
      if (/microsoft/i.test(v)) {
        return { name: 'wsl', status: 'info', detail: 'running inside WSL (windows launcher)' };
      }
    } catch {
      // not WSL
    }
  }
  return null;
}

async function checkProfiles(deps: DoctorDeps): Promise<CheckResult> {
  try {
    const file = await loadProfiles(deps.profilesFile);
    const n = file.profiles.length;
    return {
      name: 'profiles',
      status: n > 0 ? 'ok' : 'info',
      detail: n > 0 ? `${n} configured (${deps.profilesFile})` : `none yet (${deps.profilesFile})`,
    };
  } catch (err) {
    const detail = err instanceof ConfigError ? err.message : String(err);
    return { name: 'profiles', status: 'fail', detail };
  }
}

export async function runDoctor(deps: DoctorDeps): Promise<CheckResult[]> {
  const results = await Promise.all([
    Promise.resolve(checkNode(deps.nodeVersion)),
    checkTool(deps, 'git', ['--version'], true),
    checkTool(deps, 'claude', ['--version'], false),
    checkTool(deps, 'codex', ['--version'], false),
    detectWsl(deps),
    checkProfiles(deps),
  ]);
  return results.filter((r): r is CheckResult => r !== null);
}

const ICON: Record<CheckStatus, string> = { ok: '✔', warn: '!', fail: '✖', info: '·' };

export function formatResults(results: readonly CheckResult[]): string {
  const width = Math.max(...results.map((r) => r.name.length));
  return results.map((r) => `${ICON[r.status]} ${r.name.padEnd(width)}  ${r.detail}`).join('\n');
}

export function hasFailures(results: readonly CheckResult[]): boolean {
  return results.some((r) => r.status === 'fail');
}
