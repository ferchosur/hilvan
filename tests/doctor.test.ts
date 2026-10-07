import { describe, expect, it } from 'vitest';
import { checkNode, type DoctorDeps, hasFailures, runDoctor } from '../src/cli/doctor.js';
import type { RunResult } from '../src/process/run.js';

function deps(over: Partial<DoctorDeps> & { tools?: Record<string, RunResult> } = {}): DoctorDeps {
  const tools = over.tools ?? {};
  return {
    platform: 'linux',
    nodeVersion: '22.22.1',
    profilesFile: '/nonexistent/profiles.yaml',
    run: async (cmd) => tools[cmd] ?? { code: null, stdout: '', stderr: '', error: 'not_found' },
    readText: async () => 'Linux version 6.0 (generic)',
    configPerms: async () => null,
    ...over,
  };
}

const ok = (stdout: string): RunResult => ({ code: 0, stdout, stderr: '' });

describe('doctor', () => {
  it('checks the minimum Node version', () => {
    expect(checkNode('22.12.0').status).toBe('ok');
    expect(checkNode('24.1.0').status).toBe('ok');
    expect(checkNode('22.11.0').status).toBe('fail');
    expect(checkNode('20.18.0').status).toBe('fail');
  });

  it('fails without git but only warns without agent CLIs', async () => {
    const results = await runDoctor(deps());
    const byName = Object.fromEntries(results.map((r) => [r.name, r.status]));
    expect(byName.git).toBe('fail');
    expect(byName.claude).toBe('warn');
    expect(byName.codex).toBe('warn');
    expect(byName.profiles).toBe('info');
    expect(hasFailures(results)).toBe(true);
  });

  it('passes with git and reports detected versions', async () => {
    const results = await runDoctor(
      deps({ tools: { git: ok('git version 2.53.0\n'), claude: ok('2.1.292 (Claude Code)\n') } }),
    );
    expect(hasFailures(results)).toBe(false);
    expect(results.find((r) => r.name === 'claude')?.detail).toBe('2.1.292 (Claude Code)');
  });

  it('fails when profiles.yaml is writable by others (SR-005)', async () => {
    const results = await runDoctor(
      deps({ configPerms: async () => 'x is writable by group/others (666)' }),
    );
    expect(results.find((r) => r.name === 'profiles')?.status).toBe('fail');
  });

  it('detects running inside WSL', async () => {
    const results = await runDoctor(
      deps({ readText: async () => 'Linux version 5.15 microsoft-standard-WSL2' }),
    );
    expect(results.find((r) => r.name === 'wsl')?.detail).toMatch(/inside WSL/);
  });
});

describe('lazy command loading', () => {
  it('loads only the group being run', async () => {
    const { selectGroups } = await import('../src/cli/index.js');
    expect(selectGroups(['node', 'hilvan', 'criterio', 'ask', 'x'])).toEqual(['criterio']);
    expect(selectGroups(['node', 'hilvan', '--version'])).toEqual([]);
    expect(selectGroups(['node', 'hilvan', '--help'])).toEqual([
      'doctor',
      'profile',
      'criterio',
      'audit',
      'policy',
      'init',
      'task',
      'handoff',
      'review',
      'accept',
    ]);
    expect(selectGroups(['node', 'hilvan'])).toHaveLength(10);
  });
});
