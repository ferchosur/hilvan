import { readFileSync } from 'node:fs';
import { mkdir, stat } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { type AuthStatus, adapterFor } from '../adapters/index.js';
import type { Profile, ProfilesFile } from '../domain/index.js';
import { buildInvocation, type HostContext, type Invocation } from '../launchers/index.js';
import type { RunResult } from '../process/run.js';

export function currentHost(): HostContext {
  let insideWsl = false;
  if (process.platform === 'linux') {
    try {
      insideWsl = /microsoft/i.test(readFileSync('/proc/version', 'utf8'));
    } catch {
      // not WSL
    }
  }
  return { platform: process.platform, insideWsl, env: process.env };
}

/** Expands a leading `~` for paths on the local host. */
export function expandHome(p: string, home = os.homedir()): string {
  if (p === '~') return home;
  if (p.startsWith('~/') || p.startsWith('~\\')) return path.join(home, p.slice(2));
  return p;
}

/** Default isolated directory for a new profile: `~/.hilvan-profiles/<id>`. */
export function defaultConfigDir(id: string, home = os.homedir()): string {
  return path.join(home, '.hilvan-profiles', id);
}

export class ProfileError extends Error {
  override name = 'ProfileError';
}

/** Adds a profile, rejecting duplicate ids and two profiles sharing a config dir (NFR-008). */
export function addProfile(file: ProfilesFile, profile: Profile): ProfilesFile {
  if (file.profiles.some((p) => p.id === profile.id)) {
    throw new ProfileError(`a profile with id "${profile.id}" already exists`);
  }
  const clash = file.profiles.find(
    (p) =>
      p.app === profile.app &&
      p.destination.launcher === profile.destination.launcher &&
      (p.destination.distro ?? '') === (profile.destination.distro ?? '') &&
      (p.configDir === null || profile.configDir === null
        ? p.configDir === profile.configDir
        : path.normalize(p.configDir) === path.normalize(profile.configDir)),
  );
  if (clash) {
    throw new ProfileError(
      `profile "${clash.id}" already uses ${profile.configDir ?? "the CLI's default login"}; each profile needs its own config dir`,
    );
  }
  return { ...file, profiles: [...file.profiles, profile] };
}

export function removeProfile(file: ProfilesFile, id: string): ProfilesFile {
  if (!file.profiles.some((p) => p.id === id)) throw new ProfileError(`no profile with id "${id}"`);
  return { ...file, profiles: file.profiles.filter((p) => p.id !== id) };
}

export function findProfile(file: ProfilesFile, id: string): Profile {
  const p = file.profiles.find((x) => x.id === id);
  if (!p) throw new ProfileError(`no profile with id "${id}"`);
  return p;
}

export function describeDestination(p: Profile): string {
  const d = p.destination;
  return d.launcher === 'wsl' ? `wsl:${d.distro}` : d.launcher;
}

/** Problems with a native config dir that would break isolation (T10 in the threat model). */
export interface DirFindings {
  issues: string[];
  warnings: string[];
}

export async function configDirIssues(
  dir: string,
  platform = process.platform,
): Promise<DirFindings> {
  const issues: string[] = [];
  const warnings: string[] = [];
  const tmp = os.tmpdir();
  if (dir === tmp || dir.startsWith(tmp + path.sep) || dir.startsWith('/tmp/')) {
    issues.push('config dir is under the temporary directory');
  }
  try {
    const s = await stat(dir);
    if (!s.isDirectory()) issues.push('config dir is not a directory');
    else if (platform !== 'win32' && (s.mode & 0o077) !== 0) {
      warnings.push(`config dir permissions are ${(s.mode & 0o777).toString(8)}; 700 recommended`);
    }
  } catch {
    issues.push('config dir does not exist (run `hilvan profile login`)');
  }
  return { issues, warnings };
}

export async function ensureConfigDir(dir: string): Promise<void> {
  await mkdir(dir, { recursive: true, mode: 0o700 });
}

export interface ProfileCheck {
  profileId: string;
  app: Profile['app'];
  destination: string;
  configDir: string | null;
  auth: AuthStatus;
  issues: string[];
  warnings: string[];
}

export interface CheckDeps {
  host: HostContext;
  run: (inv: Invocation) => Promise<RunResult>;
  configDirIssues: (dir: string) => Promise<DirFindings>;
}

/** Verifies executable, config dir and active identity of a profile (AC-017). */
export async function checkProfile(profile: Profile, deps: CheckDeps): Promise<ProfileCheck> {
  const adapter = adapterFor(profile.app);
  const base: Omit<ProfileCheck, 'auth'> = {
    profileId: profile.id,
    app: profile.app,
    destination: describeDestination(profile),
    configDir: profile.configDir,
    issues: [],
    warnings: [],
  };

  // Directory checks only apply to paths on this host.
  if (profile.destination.launcher === 'native' && profile.configDir) {
    const dir = await deps.configDirIssues(profile.configDir);
    base.issues.push(...dir.issues);
    base.warnings.push(...dir.warnings);
  }
  if (profile.auth.mode === 'api-key' && !deps.host.env[profile.auth.apiKeyEnv]) {
    base.issues.push(`environment variable ${profile.auth.apiKeyEnv} is not set`);
  }

  let inv: Invocation;
  try {
    inv = buildInvocation(
      profile.destination,
      adapter.authStatusCommand(profile, deps.host.env),
      deps.host,
    );
  } catch (err) {
    base.issues.push((err as Error).message);
    return { ...base, auth: { loggedIn: false, verifiable: false } };
  }

  const r = await deps.run(inv);
  if (r.error === 'not_found') {
    base.issues.push(`${inv.command} not found in PATH`);
    return { ...base, auth: { loggedIn: false, verifiable: false } };
  }
  if (r.error === 'timeout') {
    base.issues.push('auth status timed out');
    return { ...base, auth: { loggedIn: false, verifiable: false } };
  }

  const auth = adapter.parseAuthStatus(r.stdout, r.stderr, r.code);
  if (!auth.loggedIn) base.issues.push('not logged in (run `hilvan profile login`)');
  if (auth.loggedIn && !auth.verifiable) base.warnings.push('active account cannot be verified');
  if (profile.auth.mode === 'subscription' && auth.method && /api|console/i.test(auth.method)) {
    base.warnings.push(`profile expects a subscription login but the CLI reports "${auth.method}"`);
  }
  return { ...base, auth };
}

/** Two profiles of the same app logged into the same account defeat isolation (AC-002). */
export function sharedAccountWarnings(checks: readonly ProfileCheck[]): string[] {
  const byKey = new Map<string, string[]>();
  for (const c of checks) {
    if (!c.auth.loggedIn || !c.auth.account) continue;
    const key = `${c.app}\u0000${c.auth.account.toLowerCase()}`;
    byKey.set(key, [...(byKey.get(key) ?? []), c.profileId]);
  }
  return [...byKey.values()]
    .filter((ids) => ids.length > 1)
    .map((ids) => `profiles ${ids.join(', ')} are logged into the same account`);
}
