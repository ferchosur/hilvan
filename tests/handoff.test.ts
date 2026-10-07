import { execFileSync } from 'node:child_process';
import { mkdtemp, readFile, rm, stat } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { approveHandoff, cancelHandoff, draftHandoff, sha256 } from '../src/core/handoff.js';
import { buildPrompt, createTask, runTask } from '../src/core/tasks.js';
import { ProfileSchema } from '../src/domain/index.js';
import { mergePolicies } from '../src/policy/evaluate.js';
import { verifyAudit } from '../src/store/audit.js';
import { ProjectStore } from '../src/store/project.js';

const FAKE = path.join(import.meta.dirname, 'fixtures', 'fake-agent.mjs');

describe.runIf(process.platform !== 'win32')('hand-off between profiles (M4)', () => {
  let tmp: string;
  let store: ProjectStore;
  const mk = (id: string, app: 'claude-code' | 'codex', over: Record<string, unknown> = {}) =>
    ProfileSchema.parse({
      id,
      label: id,
      app,
      auth: { mode: 'subscription' },
      configDir: path.join(tmp, id),
      destination: { launcher: 'native', executable: FAKE },
      accepts: ['public', 'personal', 'corporate'],
      tags: ['personal'],
      ...over,
    });
  const host = (scenario: string) => ({
    platform: process.platform,
    insideWsl: false,
    env: { ...process.env, FAKE_SCENARIO: scenario },
  });

  beforeEach(async () => {
    tmp = await mkdtemp(path.join(os.tmpdir(), 'hilvan-ho-'));
    const repo = path.join(tmp, 'repo');
    execFileSync('git', ['init', '-q', '-b', 'main', repo]);
    execFileSync('git', [
      '-C',
      repo,
      '-c',
      'user.name=t',
      '-c',
      'user.email=t@t',
      'commit',
      '-q',
      '--allow-empty',
      '-m',
      'i',
    ]);
    store = new ProjectStore(repo);
    await store.init({ name: 'repo', baseBranch: 'main' });
  });
  afterEach(async () => {
    await rm(tmp, { recursive: true, force: true });
  });

  it('continues a rate-limited task on another profile with an approved package (AC-005, AC-007)', async () => {
    const a = mk('claude-a', 'claude-code');
    const b = mk('codex-b', 'codex');
    const t = await createTask(store, {
      title: 'Add hello file',
      objective: 'Create hello.txt',
      classification: 'personal',
      mode: 'write',
    });
    await runTask(store, t.id, a, { host: host('write') }); // makes a commit
    // second run hits the limit
    const blocked = await runTask(store, t.id, a, { host: host('limit') });
    expect(blocked.task.state).toBe('blocked');

    const draft = await draftHandoff(store, t.id, b, a, mergePolicies({}));
    expect(draft.evaluation.eligible).toBe(true);
    expect(draft.body).toMatch(/## Objetivo/);
    expect(draft.body).toMatch(/bloqueada por rate_limit/);
    expect(draft.body).toMatch(/NO verificados/);
    expect(draft.body).toMatch(/commit [0-9a-f]+ hilvan\(/);
    expect(draft.body).toMatch(/hello\.txt/);
    expect(draft.body).toContain('[REDACTED:anthropic-key]'); // fake agent summary contained a key
    expect(draft.secrets.length).toBeGreaterThan(0);

    const edited = draft.body.replace(
      '_(añada aquí decisiones tomadas que el siguiente agente deba respetar)_',
      'Usar UTF-8',
    );
    const r = await approveHandoff(store, draft, edited);
    expect(r.hash).toBe(sha256(edited));
    expect(r.task.profileId).toBe('codex-b');
    expect(r.task.state).toBe('ready');
    const stored = await readFile(r.file, 'utf8');
    expect(stored).toContain(`sha256: ${r.hash}`);
    expect(stored).toContain('Usar UTF-8');
    expect((await stat(r.file)).mode & 0o777).toBe(0o600);

    // the new profile receives the package in its prompt and does not resume the other's session
    const prompt = buildPrompt(
      r.task,
      await store.brief(t.id),
      '/wt',
      await store.handoff(t.id, draft.id),
    );
    expect(prompt).toContain('Paquete de continuidad');
    expect(prompt).toContain('Treat "assumptions" and agent claims as unverified');
    const next = await runTask(store, t.id, b, { host: host('success') });
    expect(next.run.profileId).toBe('codex-b');

    const audit = await readFile(path.join(store.dir, 'events.jsonl'), 'utf8');
    expect(audit).toContain('"handoff.approved"');
    expect(audit).toContain(r.hash);
    expect(audit).not.toContain('Usar UTF-8'); // audit keeps hashes, not content (SR-018)
    expect((await verifyAudit(path.join(store.dir, 'events.jsonl'))).ok).toBe(true);
  });

  it('cancelling sends nothing and keeps the original profile (AC-004)', async () => {
    const a = mk('claude-a', 'claude-code');
    const b = mk('codex-b', 'codex');
    const t = await createTask(store, { title: 'x', classification: 'personal', mode: 'read' });
    await runTask(store, t.id, a, { host: host('limit') });
    const draft = await draftHandoff(store, t.id, b, a, mergePolicies({}));
    await cancelHandoff(store, draft);
    const after = await store.loadTask(t.id);
    expect(after.profileId).toBe('claude-a');
    expect(after.handoffs).toEqual([]);
    await expect(runTask(store, t.id, b, { host: host('success') })).rejects.toThrow(
      /needs a handoff/,
    );
  });

  it('refuses ineligible targets and edited packages that reintroduce secrets', async () => {
    const a = mk('claude-a', 'claude-code');
    const corpOnly = mk('corp', 'claude-code', { accepts: ['corporate'] });
    const t = await createTask(store, { title: 'x', classification: 'personal', mode: 'read' });
    await runTask(store, t.id, a, { host: host('success') });
    const draft = await draftHandoff(store, t.id, corpOnly, a, mergePolicies({}));
    expect(draft.evaluation.eligible).toBe(false);
    await expect(approveHandoff(store, draft, draft.body)).rejects.toThrow(/not eligible/);

    const ok = await draftHandoff(store, t.id, mk('codex-b', 'codex'), a, mergePolicies({}));
    await expect(approveHandoff(store, ok, `${ok.body}\nAKIAIOSFODNN7EXAMPLE`)).rejects.toThrow(
      /possible secrets/,
    );
  });

  it('an enforced managed rule blocks the hand-off (AC-014)', async () => {
    const a = mk('claude-corp', 'claude-code', { tags: ['corporate'] });
    const b = mk('claude-personal', 'claude-code');
    const t = await createTask(store, { title: 'x', classification: 'corporate', mode: 'read' });
    await runTask(store, t.id, a, { host: host('success') });
    const managed = mergePolicies({
      managed: {
        version: 1,
        rules: [
          {
            id: 'no-corp-out',
            from: { tags: ['corporate'] },
            to: { tags: ['personal'] },
            effect: 'deny',
            enforced: true,
            reason: 'company policy',
          },
        ],
      },
    });
    const draft = await draftHandoff(store, t.id, b, a, managed);
    expect(draft.evaluation.eligible).toBe(false);
    expect(draft.evaluation.exclusions[0]).toMatchObject({
      enforced: true,
      reason: 'company policy',
    });
  });
});
