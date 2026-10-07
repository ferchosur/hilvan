import { mkdtemp, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { buildProfile } from '../src/cli/profile.js';
import { loadProfiles, saveProfiles } from '../src/config/profiles.js';
import {
  addProfile,
  type CheckDeps,
  checkProfile,
  expandHome,
  ProfileError,
  removeProfile,
  sharedAccountWarnings,
} from '../src/core/profiles.js';
import type { ProfilesFile } from '../src/domain/index.js';
import type { RunResult } from '../src/process/run.js';

const empty: ProfilesFile = { version: 1, profiles: [] };
const mk = (id: string, over: Record<string, unknown> = {}) =>
  buildProfile({
    id,
    app: 'claude-code',
    accepts: ['personal'],
    configDir: `/home/u/.hilvan-profiles/${id}`,
    ...over,
  });

describe('profile registry', () => {
  it('supports two profiles of the same app (AC-001)', () => {
    const f = addProfile(addProfile(empty, mk('claude-corp')), mk('claude-personal'));
    expect(f.profiles.map((p) => p.id)).toEqual(['claude-corp', 'claude-personal']);
  });

  it('rejects two profiles sharing a config dir (AC-017)', () => {
    const f = addProfile(empty, mk('a'));
    expect(() => addProfile(f, mk('b', { configDir: '/home/u/.hilvan-profiles/a' }))).toThrow(
      ProfileError,
    );
  });

  it('allows only one default-login profile per app and destination', () => {
    const f = addProfile(empty, mk('a', { configDir: undefined, useDefaultLogin: true }));
    expect(() => addProfile(f, mk('b', { configDir: undefined, useDefaultLogin: true }))).toThrow(
      /default login/,
    );
    expect(
      addProfile(f, mk('c', { app: 'codex', configDir: undefined, useDefaultLogin: true }))
        .profiles,
    ).toHaveLength(2);
  });

  it('removes profiles and reports unknown ids', () => {
    const f = addProfile(empty, mk('a'));
    expect(removeProfile(f, 'a').profiles).toEqual([]);
    expect(() => removeProfile(f, 'zzz')).toThrow(ProfileError);
  });

  it('defaults to an isolated dir and can reuse the CLI default login', () => {
    const iso = buildProfile({ id: 'x', app: 'codex', accepts: ['public'] });
    expect(iso.configDir).toBe(path.join(os.homedir(), '.hilvan-profiles', 'x'));
    const def = buildProfile({ id: 'y', app: 'codex', accepts: ['public'], useDefaultLogin: true });
    expect(def.configDir).toBeNull();
  });

  it('expands ~', () => {
    expect(expandHome('~/a', '/h')).toBe(path.join('/h', 'a'));
    expect(expandHome('/abs', '/h')).toBe('/abs');
  });
});

describe('profiles.yaml persistence', () => {
  let dir: string;
  beforeEach(async () => {
    dir = await mkdtemp(path.join(os.tmpdir(), 'hilvan-'));
  });
  afterEach(async () => {
    await rm(dir, { recursive: true, force: true });
  });

  it('round-trips through YAML', async () => {
    const file = path.join(dir, 'cfg', 'profiles.yaml');
    const data = addProfile(addProfile(empty, mk('a')), mk('b', { app: 'codex' }));
    await saveProfiles(file, data);
    expect(await loadProfiles(file)).toEqual(data);
  });
});

describe('checkProfile', () => {
  const deps = (r: RunResult, issues: string[] = []): CheckDeps => ({
    host: { platform: 'linux', insideWsl: false, env: {} },
    run: async () => r,
    configDirIssues: async () => ({ issues, warnings: [] }),
  });

  it('reports the active account', async () => {
    const out = JSON.stringify({ loggedIn: true, authMethod: 'claude.ai', email: 'a@x.io' });
    const c = await checkProfile(mk('a'), deps({ code: 0, stdout: out, stderr: '' }));
    expect(c.issues).toEqual([]);
    expect(c.auth.account).toBe('a@x.io');
  });

  it('flags missing executables and logins', async () => {
    const missing = await checkProfile(
      mk('a'),
      deps({ code: null, stdout: '', stderr: '', error: 'not_found' }),
    );
    expect(missing.issues[0]).toMatch(/not found/);
    const out = JSON.stringify({ loggedIn: false });
    const loggedOut = await checkProfile(mk('a'), deps({ code: 0, stdout: out, stderr: '' }));
    expect(loggedOut.issues.join()).toMatch(/not logged in/);
  });

  it('warns when Codex cannot verify the account', async () => {
    const c = await checkProfile(
      mk('c', { app: 'codex' }),
      deps({ code: 0, stdout: 'Logged in using ChatGPT', stderr: '' }),
    );
    expect(c.warnings).toContain('active account cannot be verified');
  });

  it('flags api-key profiles whose variable is not set', async () => {
    const p = mk('k', { auth: 'api-key', apiKeyEnv: 'WORK_KEY' });
    const out = JSON.stringify({ loggedIn: true, authMethod: 'api_key' });
    const c = await checkProfile(p, deps({ code: 0, stdout: out, stderr: '' }));
    expect(c.issues.join()).toMatch(/WORK_KEY/);
  });

  it('warns when two profiles share an account', () => {
    const base = {
      app: 'claude-code' as const,
      destination: 'native',
      configDir: '',
      issues: [],
      warnings: [],
    };
    const auth = { loggedIn: true, verifiable: true, account: 'A@x.io' };
    const w = sharedAccountWarnings([
      { ...base, profileId: 'a', auth },
      { ...base, profileId: 'b', auth: { ...auth, account: 'a@x.io' } },
    ]);
    expect(w).toEqual(['profiles a, b are logged into the same account']);
  });
});
