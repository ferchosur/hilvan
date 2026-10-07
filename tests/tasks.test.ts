import { execFileSync } from 'node:child_process';
import { mkdtemp, readFile, rm, stat } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  cancelTask,
  createTask,
  discardTask,
  recoverStaleRun,
  runTask,
  suggest,
} from '../src/core/tasks.js';
import { Criterio } from '../src/criterio/index.js';
import { ProfileSchema } from '../src/domain/index.js';
import { verifyAudit } from '../src/store/audit.js';
import { ProjectStore } from '../src/store/project.js';

const posix = process.platform !== 'win32';
const FAKE = path.join(import.meta.dirname, 'fixtures', 'fake-agent.mjs');

function git(cwd: string, ...args: string[]) {
  return execFileSync('git', args, { cwd, encoding: 'utf8' }).trim();
}

describe.runIf(posix)('task lifecycle with a fake agent (M2)', () => {
  let tmp: string;
  let repo: string;
  let store: ProjectStore;
  const profile = (scenario = 'write', over: Record<string, unknown> = {}) =>
    ProfileSchema.parse({
      id: 'fake',
      label: 'fake',
      app: 'claude-code',
      auth: { mode: 'subscription' },
      configDir: path.join(tmp, 'cfg'),
      destination: { launcher: 'native', executable: FAKE },
      accepts: ['personal', 'public'],
      ...over,
    });
  const host = (scenario: string) => ({
    platform: process.platform,
    insideWsl: false,
    env: { ...process.env, FAKE_SCENARIO: scenario },
  });

  beforeEach(async () => {
    tmp = await mkdtemp(path.join(os.tmpdir(), 'hilvan-m2-'));
    repo = path.join(tmp, 'repo');
    execFileSync('git', ['init', '-q', '-b', 'main', repo]);
    git(
      repo,
      '-c',
      'user.name=t',
      '-c',
      'user.email=t@t',
      'commit',
      '-q',
      '--allow-empty',
      '-m',
      'init',
    );
    store = new ProjectStore(repo);
    await store.init({ name: 'repo', baseBranch: 'main' });
  });
  afterEach(async () => {
    await rm(tmp, { recursive: true, force: true });
  });

  it('keeps .hilvan out of git (SR-016)', async () => {
    expect(git(repo, 'status', '--porcelain')).toBe('');
    expect(await readFile(path.join(repo, '.hilvan', '.gitignore'), 'utf8')).toBe('*\n');
  });

  it('runs a write task in its own worktree, commits, measures and audits (AC-016, AC-021)', async () => {
    const criterio = new Criterio();
    const t = await createTask(store, {
      title: 'Add hello file',
      criteria: ['hello.txt exists'],
      classification: 'personal',
      mode: 'write',
    });
    const r = await runTask(store, t.id, profile(), { host: host('write'), criterio });
    expect(r.run.state).toBe('completed');
    expect(r.task.state).toBe('in_review');
    expect(r.task.branch).toBe(`hilvan/${t.id}`);
    expect(r.run.diff).toMatchObject({ filesChanged: 1, paths: ['hello.txt'] });
    expect(r.run.risk).toBeGreaterThanOrEqual(2);
    expect(r.run.usage?.costUsd).toBeCloseTo(0.0123);
    expect(r.run.limits?.[0]).toMatchObject({ name: 'five_hour', utilization: 0.4 });
    // main is untouched; the change lives on the task branch
    expect(git(repo, 'ls-tree', '--name-only', 'main')).not.toContain('hello.txt');
    expect(git(repo, 'ls-tree', '--name-only', `hilvan/${t.id}`)).toContain('hello.txt');
    // run log is stored with secrets redacted (SR-013)
    const log = await readFile(store.runFile(t.id, r.run.id), 'utf8');
    expect(log).toContain('[REDACTED:anthropic-key]');
    expect(log).not.toContain('sk-ant-api03');
    expect((await stat(store.runFile(t.id, r.run.id))).mode & 0o777).toBe(0o600);
    // audit chain intact
    const v = await verifyAudit(path.join(store.dir, 'events.jsonl'));
    expect(v.ok).toBe(true);
    expect(v.events).toBeGreaterThanOrEqual(3);
  });

  it('blocks on a usage limit and refuses to switch profile silently (AC-007)', async () => {
    const t = await createTask(store, { title: 'x', classification: 'personal', mode: 'read' });
    const r = await runTask(store, t.id, profile(), { host: host('limit') });
    expect(r.task.state).toBe('blocked');
    expect(r.run.blockedReason).toBe('rate_limit');
    await expect(
      runTask(store, t.id, profile('success', { id: 'other' }), { host: host('success') }),
    ).rejects.toThrow(/needs a handoff/);
  });

  it('refuses a profile that does not accept the classification (AC-012 minimal)', async () => {
    const t = await createTask(store, { title: 'x', classification: 'corporate', mode: 'read' });
    await expect(runTask(store, t.id, profile(), { host: host('success') })).rejects.toThrow(
      /does not accept "corporate"/,
    );
  });

  it('marks an interrupted run as uncertain on recovery (AC-020)', async () => {
    const t = await createTask(store, { title: 'x', classification: 'personal', mode: 'read' });
    await store.saveTask({ ...(await store.loadTask(t.id)), state: 'running', profileId: 'fake' });
    await store.acquireLock({ taskId: t.id, runId: 'run_01JABCDEFGHJKMNPQRSTVWXYZ0' });
    // forge a dead pid
    const lock = JSON.parse(await readFile(store.lockFile, 'utf8'));
    const { writeFile } = await import('node:fs/promises');
    await writeFile(store.lockFile, JSON.stringify({ ...lock, pid: 999999 }));
    expect(await recoverStaleRun(store)).toBe(t.id);
    expect((await store.loadTask(t.id)).state).toBe('uncertain');
    expect(await store.readLock()).toBeNull();
  });

  it('only one run at a time per project', async () => {
    const t = await createTask(store, { title: 'x', classification: 'personal', mode: 'read' });
    await store.acquireLock({ taskId: t.id, runId: 'run_01JABCDEFGHJKMNPQRSTVWXYZ0' });
    await expect(runTask(store, t.id, profile(), { host: host('success') })).rejects.toThrow(
      /another run is active/,
    );
    expect(await cancelTask(store, 'tsk_01JABCDEFGHJKMNPQRSTVWXYZ0')).toBe(false);
  });

  it('discards a task with its worktree and branch', async () => {
    const t = await createTask(store, { title: 'x', classification: 'personal', mode: 'write' });
    await runTask(store, t.id, profile(), { host: host('write') });
    await discardTask(store, t.id);
    expect(git(repo, 'branch', '--list', `hilvan/${t.id}`)).toBe('');
    expect((await store.loadTask(t.id)).state).toBe('discarded');
  });

  it('redacts secrets typed into a brief and resolves short ids', async () => {
    const t = await createTask(store, {
      title: 'deploy',
      objective: 'use key AKIAIOSFODNN7EXAMPLE',
      classification: 'personal',
      mode: 'read',
    });
    expect(await store.brief(t.id)).toContain('[REDACTED:aws-access-key]');
    expect(await store.resolveTaskId(t.id.slice(-6))).toBe(t.id);
  });

  it('Criterio suggests type, complexity and classification for a brief', () => {
    const s = suggest(new Criterio(), {
      title: 'Corregir bug del login',
      objective: 'Datos de clientes en Jira interno',
      classification: 'personal',
      mode: 'write',
    });
    expect(s.taskType.value).toBe('fix');
    expect(s.classification.value).toBe('corporate');
  });
});

describe('SR-012 repository hooks trust', () => {
  it('flags hooks until trusted, and again when they change', async () => {
    const { mkdir, writeFile } = await import('node:fs/promises');
    const { repoAgentConfig, trust, untrusted } = await import('../src/core/repo-trust.js');
    const root = await mkdtemp(path.join(os.tmpdir(), 'hilvan-trust-'));
    const state = path.join(root, '.hilvan');
    await mkdir(path.join(root, '.claude'), { recursive: true });
    await writeFile(
      path.join(root, '.claude', 'settings.json'),
      '{"hooks":{"Stop":[{"command":"curl x"}]}}',
    );
    await writeFile(path.join(root, '.mcp.json'), '{"mcpServers":{}}');
    const items = await repoAgentConfig(root);
    expect(items.map((i) => i.file).sort()).toEqual(['.claude/settings.json', '.mcp.json']);
    expect(await untrusted(state, items)).toHaveLength(1);
    await trust(state, await untrusted(state, items));
    expect(await untrusted(state, await repoAgentConfig(root))).toHaveLength(0);
    await writeFile(
      path.join(root, '.claude', 'settings.json'),
      '{"hooks":{"Stop":[{"command":"rm -rf x"}]}}',
    );
    expect(await untrusted(state, await repoAgentConfig(root))).toHaveLength(1);
    await rm(root, { recursive: true, force: true });
  });
});
