import { describe, expect, it } from 'vitest';
import { adapterFor } from '../src/adapters/index.js';
import { ProfileSchema } from '../src/domain/index.js';

const claude = adapterFor('claude-code');
const codex = adapterFor('codex');

const profile = (over: Record<string, unknown> = {}) =>
  ProfileSchema.parse({
    id: 'p',
    label: 'p',
    app: 'claude-code',
    auth: { mode: 'subscription' },
    configDir: '/home/u/.hilvan-profiles/p',
    destination: { launcher: 'native' },
    accepts: ['personal'],
    ...over,
  });

describe('claude-code adapter', () => {
  it('parses a logged-in status (shape from `claude auth status --json`)', () => {
    const out = JSON.stringify({
      loggedIn: true,
      authMethod: 'claude.ai',
      email: 'a@example.com',
      orgName: "a@example.com's Organization",
    });
    expect(claude.parseAuthStatus(out, '', 0)).toEqual({
      loggedIn: true,
      verifiable: true,
      method: 'claude.ai',
      account: 'a@example.com',
    });
  });

  it('parses a logged-out status', () => {
    const s = claude.parseAuthStatus(
      JSON.stringify({ loggedIn: false, authMethod: 'none' }),
      '',
      0,
    );
    expect(s.loggedIn).toBe(false);
  });

  it('reports unreadable output instead of crashing', () => {
    expect(claude.parseAuthStatus('oops', 'boom', 1).loggedIn).toBe(false);
  });

  it('isolates subscription profiles from stray API keys', () => {
    const { env, unsetEnv } = claude.profileEnv(profile(), { ANTHROPIC_API_KEY: 'x' });
    expect(env).toEqual({ CLAUDE_CONFIG_DIR: '/home/u/.hilvan-profiles/p' });
    expect(unsetEnv).toContain('ANTHROPIC_API_KEY');
  });

  it('maps an api-key profile variable without storing the value', () => {
    const p = profile({ auth: { mode: 'api-key', apiKeyEnv: 'WORK_KEY' } });
    expect(claude.profileEnv(p, { WORK_KEY: 'secret' }).env.ANTHROPIC_API_KEY).toBe('secret');
    expect(JSON.stringify(p)).not.toContain('secret');
  });

  it('removes CLAUDE_CONFIG_DIR for the default login instead of pointing it at ~/.claude', () => {
    const { env, unsetEnv } = claude.profileEnv(profile({ configDir: null }), {});
    expect(env.CLAUDE_CONFIG_DIR).toBeUndefined();
    expect(unsetEnv).toContain('CLAUDE_CONFIG_DIR');
  });

  it('logs in with the official flow', () => {
    expect(claude.loginCommand(profile(), {}).args).toEqual(['auth', 'login']);
  });
});

describe('codex adapter', () => {
  it('parses a ChatGPT login as unverifiable account', () => {
    const s = codex.parseAuthStatus('Logged in using ChatGPT\n', '', 0);
    expect(s).toMatchObject({ loggedIn: true, method: 'chatgpt', verifiable: false });
  });

  it('parses "Not logged in"', () => {
    expect(codex.parseAuthStatus('Not logged in\n', '', 1).loggedIn).toBe(false);
  });

  it('reports a missing CODEX_HOME', () => {
    const s = codex.parseAuthStatus('', 'Error loading configuration: CODEX_HOME points to "x"', 1);
    expect(s.loggedIn).toBe(false);
    expect(s.detail).toMatch(/CODEX_HOME/);
  });

  it('sets CODEX_HOME per profile', () => {
    const p = profile({ app: 'codex', configDir: '/home/u/.hilvan-profiles/cx' });
    expect(codex.profileEnv(p, {}).env).toEqual({ CODEX_HOME: '/home/u/.hilvan-profiles/cx' });
  });
});
