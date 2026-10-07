import { execFileSync } from 'node:child_process';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  acceptTask,
  crossReview,
  learnOutcome,
  parseVerdict,
  reviewDiff,
} from '../src/core/review.js';
import { createTask, discardTask, runTask } from '../src/core/tasks.js';
import { Criterio } from '../src/criterio/index.js';
import { ProfileSchema } from '../src/domain/index.js';
import { ProjectStore } from '../src/store/project.js';

const FAKE = path.join(import.meta.dirname, 'fixtures', 'fake-agent.mjs');
const git = (cwd: string, ...a: string[]) =>
  execFileSync('git', a, { cwd, encoding: 'utf8' }).trim();

describe('verdict parsing', () => {
  it('reads the final verdict line', () => {
    expect(parseVerdict('ok\nVERDICT: approve')).toBe('approve');
    expect(parseVerdict('VERDICT: Changes_Requested')).toBe('changes_requested');
    expect(parseVerdict('looks fine')).toBe('unknown');
  });
});

describe.runIf(process.platform !== 'win32')('review, cross review and accept (M5)', () => {
  let tmp: string;
  let repo: string;
  let store: ProjectStore;
  const mk = (id: string, app: 'claude-code' | 'codex' = 'claude-code') =>
    ProfileSchema.parse({
      id,
      label: id,
      app,
      auth: { mode: 'subscription' },
      configDir: path.join(tmp, id),
      destination: { launcher: 'native', executable: FAKE },
      accepts: ['public', 'personal'],
    });
  const host = (s: string) => ({
    platform: process.platform,
    insideWsl: false,
    env: { ...process.env, FAKE_SCENARIO: s },
  });

  beforeEach(async () => {
    tmp = await mkdtemp(path.join(os.tmpdir(), 'hilvan-m5-'));
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

  async function writtenTask() {
    const t = await createTask(store, {
      title: 'Add hello',
      classification: 'personal',
      mode: 'write',
      taskType: 'implement',
    });
    await runTask(store, t.id, mk('claude-a'), { host: host('write') });
    return t.id;
  }

  it('requires the current diff to be reviewed before accepting (SR-024)', async () => {
    const id = await writtenTask();
    await expect(acceptTask(store, id)).rejects.toThrow(/review the current diff first/);
    const { diff } = await reviewDiff(store, id);
    expect(diff).toContain('hello from the agent');
  });

  it('accepting merges into the base branch, removes the worktree and teaches Criterio (AC-010, AC-021)', async () => {
    const id = await writtenTask();
    await reviewDiff(store, id);
    const criterio = new Criterio();
    const t = await acceptTask(store, id, criterio);
    expect(t.state).toBe('accepted');
    expect(git(repo, 'ls-tree', '--name-only', 'main')).toContain('hello.txt');
    expect(git(repo, 'branch', '--list', `hilvan/${id}`)).toBe('');
    expect(criterio.stats().profiles).toEqual([{ key: 'claude-a|implement', mean: 2 / 3, n: 1 }]);
  });

  it('refuses to merge into a dirty or different checkout', async () => {
    const id = await writtenTask();
    await reviewDiff(store, id);
    await writeFile(path.join(repo, 'tracked.txt'), 'x');
    git(repo, 'add', 'tracked.txt');
    await expect(acceptTask(store, id)).rejects.toThrow(/uncommitted changes/);
    git(repo, 'reset', '-q', 'tracked.txt');
    git(repo, 'checkout', '-q', '-b', 'other');
    await expect(acceptTask(store, id)).rejects.toThrow(/check out main/);
  });

  it('a cross review by another profile is attached with its verdict (AC-018)', async () => {
    const id = await writtenTask();
    await expect(crossReview(store, id, mk('claude-a'), { host: host('review') })).rejects.toThrow(
      /different profile/,
    );
    const r = await crossReview(store, id, mk('claude-b'), { host: host('review') }); // fake agent speaks Claude's protocol
    expect(r.verdict).toBe('changes_requested');
    const t = await store.loadTask(id);
    expect(t.reviews).toEqual([
      expect.objectContaining({ profileId: 'claude-b', verdict: 'changes_requested' }),
    ]);
    expect(t.profileId).toBe('claude-a'); // reviewing never switches the task's identity
    expect(t.state).toBe('in_review');
  });

  it('discarding a reviewed task records a rejection for Criterio', async () => {
    const id = await writtenTask();
    const criterio = new Criterio();
    await discardTask(store, id, undefined, (task) => learnOutcome(task, false, criterio));
    expect(criterio.stats().profiles[0]).toMatchObject({
      key: 'claude-a|implement',
      n: 1,
      mean: 1 / 3,
    });
  });
});
