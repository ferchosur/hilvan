import { describe, expect, it } from 'vitest';
import {
  canTransition,
  isTerminal,
  ProfileSchema,
  ProfilesFileSchema,
} from '../src/domain/index.js';

const base = {
  id: 'claude-corp',
  label: 'Claude — corporate',
  app: 'claude-code',
  auth: { mode: 'subscription' },
  configDir: '/home/user/.hilvan-profiles/claude-corp',
  destination: { launcher: 'native' },
  accepts: ['corporate'],
};

describe('ProfileSchema', () => {
  it('accepts a minimal profile and fills defaults', () => {
    const p = ProfileSchema.parse(base);
    expect(p.tags).toEqual([]);
    expect(p.tools.provides).toEqual([]);
    expect(p.model).toBeNull();
    expect(p.preference).toBe(0);
  });

  it('requires a distro for the wsl launcher', () => {
    const r = ProfileSchema.safeParse({ ...base, destination: { launcher: 'wsl' } });
    expect(r.success).toBe(false);
  });

  it('requires apiKeyEnv as a variable name for api-key auth, never a value', () => {
    const ok = ProfileSchema.safeParse({
      ...base,
      auth: { mode: 'api-key', apiKeyEnv: 'WORK_KEY' },
    });
    const bad = ProfileSchema.safeParse({
      ...base,
      auth: { mode: 'api-key', apiKeyEnv: 'sk-ant-123' },
    });
    expect(ok.success).toBe(true);
    expect(bad.success).toBe(false);
  });

  it('rejects unknown apps and invalid ids', () => {
    expect(ProfileSchema.safeParse({ ...base, app: 'other' }).success).toBe(false);
    expect(ProfileSchema.safeParse({ ...base, id: 'Bad Id' }).success).toBe(false);
  });
});

describe('ProfilesFileSchema', () => {
  it('allows zero, one or many profiles of the same app (AC-011)', () => {
    expect(ProfilesFileSchema.parse({ version: 1 }).profiles).toEqual([]);
    const two = ProfilesFileSchema.parse({
      version: 1,
      profiles: [base, { ...base, id: 'claude-personal', accepts: ['personal'] }],
    });
    expect(two.profiles).toHaveLength(2);
  });

  it('rejects duplicate profile ids', () => {
    const r = ProfilesFileSchema.safeParse({ version: 1, profiles: [base, base] });
    expect(r.success).toBe(false);
  });
});

describe('task state machine', () => {
  it('never moves to another profile silently: blocked only returns to ready', () => {
    expect(canTransition('running', 'blocked')).toBe(true);
    expect(canTransition('blocked', 'running')).toBe(false);
    expect(canTransition('blocked', 'ready')).toBe(true);
  });

  it('requires review before acceptance', () => {
    expect(canTransition('running', 'accepted')).toBe(false);
    expect(canTransition('in_review', 'accepted')).toBe(true);
  });

  it('marks accepted and discarded as terminal', () => {
    expect(isTerminal('accepted')).toBe(true);
    expect(isTerminal('discarded')).toBe(true);
    expect(isTerminal('uncertain')).toBe(false);
  });
});
