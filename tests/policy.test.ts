import { execFileSync } from 'node:child_process';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { loadEffectivePolicy, loadPolicyFile } from '../src/config/policy.js';
import { quotaHeadroom, recommend } from '../src/core/recommend.js';
import { createTask, runTask } from '../src/core/tasks.js';
import { Criterio } from '../src/criterio/index.js';
import { ProfileSchema, type Task } from '../src/domain/index.js';
import { evaluate, mergePolicies } from '../src/policy/evaluate.js';
import { ProjectStore } from '../src/store/project.js';

const prof = (id: string, over: Record<string, unknown> = {}) =>
  ProfileSchema.parse({
    id,
    label: id,
    app: 'claude-code',
    auth: { mode: 'subscription' },
    configDir: `/h/${id}`,
    destination: { launcher: 'native' },
    accepts: ['public', 'personal'],
    ...over,
  });
const corp = prof('claude-corp', {
  accepts: ['public', 'corporate', 'confidential'],
  tags: ['corporate'],
  tools: { provides: ['jira'] },
});
const personal = prof('claude-personal', {
  accepts: ['public', 'personal', 'corporate'],
  tags: ['personal'],
});
const codex = prof('codex-personal', {
  app: 'codex',
  accepts: ['public', 'personal', 'corporate'],
  tags: ['personal'],
});
const task = (over: Partial<Task> = {}) =>
  ({ classification: 'corporate', requires: [], ...over }) as Task;

const approvalRule = {
  id: 'corp-to-personal',
  from: { classification: ['corporate', 'confidential'] },
  to: { tags: ['personal'] },
  effect: 'require_approval' as const,
  enforced: false,
  reason: 'corporate data to a personal account needs approval',
};
const corpToPersonal = { version: 1 as const, rules: [approvalRule] };

describe('policy evaluation (FR-011)', () => {
  it('a tool connected only to the corporate profile is not available elsewhere (AC-006)', () => {
    const p = mergePolicies({});
    expect(evaluate(p, task({ requires: ['jira'] }), corp).eligible).toBe(true);
    const ev = evaluate(p, task({ requires: ['jira'] }), personal);
    expect(ev.eligible).toBe(false);
    expect(ev.exclusions[0]?.reason).toMatch(/lacks required tools: jira/);
  });

  it('excludes profiles that do not accept the classification, with a visible reason (AC-012)', () => {
    const ev = evaluate(mergePolicies({}), task({ classification: 'confidential' }), personal);
    expect(ev).toMatchObject({ eligible: false });
    expect(ev.exclusions[0]?.reason).toMatch(/does not accept "confidential"/);
  });

  it('require_approval keeps the profile eligible but flags the transfer (FR-004)', () => {
    const ev = evaluate(mergePolicies({ user: corpToPersonal }), task(), personal);
    expect(ev).toMatchObject({ eligible: true, requiresApproval: true });
    expect(evaluate(mergePolicies({ user: corpToPersonal }), task(), corp).requiresApproval).toBe(
      false,
    );
  });

  it('managed enforced deny wins; enforced is ignored outside the managed layer (SR-020)', () => {
    const deny = { ...approvalRule, id: 'no-corp-out', effect: 'deny' as const, enforced: true };
    const managed = mergePolicies({ managed: { version: 1, rules: [deny] } });
    const ev = evaluate(managed, task(), personal);
    expect(ev.eligible).toBe(false);
    expect(ev.exclusions.some((e) => e.enforced)).toBe(true);
    const user = mergePolicies({ user: { version: 1, rules: [deny] } });
    expect(user.rules[0]?.enforced).toBe(false);
    expect(user.warnings.join()).toMatch(/only the managed policy can enforce/);
  });

  it('a project cannot redefine classifications (SR-019)', () => {
    const p = mergePolicies({ project: { version: 1, classifications: ['public'], rules: [] } });
    expect(p.classifications).toContain('confidential');
    expect(p.warnings.join()).toMatch(/cannot redefine/);
  });
});

describe('recommendation (FR-012, AC-013)', () => {
  it('ranks only eligible profiles and explains exclusions', () => {
    const rec = recommend(
      mergePolicies({}),
      { ...task({ requires: ['jira'] }), title: 'Exportar issues' } as Task,
      [corp, personal, codex],
      new Criterio(),
      [],
      1,
    );
    expect(rec.ranked.map((r) => r.profile.id)).toEqual(['claude-corp']);
    expect(rec.excluded.map((x) => x.profile.id).sort()).toEqual([
      'claude-personal',
      'codex-personal',
    ]);
  });

  it('derives quota headroom from the latest reported limits', () => {
    const hist = [
      {
        runs: [
          {
            id: 'run_01JABCDEFGHJKMNPQRSTVWXYZ0',
            profileId: 'claude-personal',
            state: 'completed',
            limits: [
              { name: 'five_hour', utilization: 0.3 },
              { name: 'seven_day', utilization: 0.8 },
            ],
            startedAt: '2026-10-07T00:00:00.000Z',
          },
        ],
      },
    ] as unknown as Task[];
    expect(quotaHeadroom('claude-personal', hist)).toBeCloseTo(0.2);
    expect(quotaHeadroom('codex-personal', hist)).toBeUndefined();
  });
});

describe('policy files', () => {
  let dir: string;
  beforeEach(async () => {
    dir = await mkdtemp(path.join(os.tmpdir(), 'hilvan-pol-'));
  });
  afterEach(async () => {
    await rm(dir, { recursive: true, force: true });
  });

  it('loads and merges layers; rejects unknown fields and allow effects', async () => {
    const user = path.join(dir, 'user.yaml');
    await writeFile(
      user,
      'version: 1\nrules:\n  - id: r1\n    from: { classification: [corporate] }\n    to: { tags: [personal] }\n    effect: require_approval\n    reason: needs approval\n',
    );
    const eff = await loadEffectivePolicy({ managed: path.join(dir, 'none.yaml'), user });
    expect(eff.rules.map((r) => `${r.layer}:${r.id}`)).toEqual(['user:r1']);
    const bad = path.join(dir, 'bad.yaml');
    await writeFile(bad, 'version: 1\nrules:\n  - id: r2\n    effect: allow\n    reason: x\n');
    await expect(loadPolicyFile(bad)).rejects.toThrow();
  });
});

describe.runIf(process.platform !== 'win32')(
  'approval gate in runTask (AC-003, AC-004, AC-014)',
  () => {
    let tmp: string;
    let store: ProjectStore;
    const FAKE = path.join(import.meta.dirname, 'fixtures', 'fake-agent.mjs');
    const host = {
      platform: process.platform,
      insideWsl: false,
      env: { ...process.env, FAKE_SCENARIO: 'success' },
    };
    const fakePersonal = () =>
      ProfileSchema.parse({
        ...personal,
        configDir: path.join(tmp, 'cfg'),
        destination: { launcher: 'native', executable: FAKE },
      });

    beforeEach(async () => {
      tmp = await mkdtemp(path.join(os.tmpdir(), 'hilvan-appr-'));
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

    it('shows the exact brief and sends nothing when the user declines', async () => {
      const t = await createTask(store, {
        title: 'Informe trimestral',
        objective: 'datos internos',
        classification: 'corporate',
        mode: 'read',
      });
      let seen = '';
      await expect(
        runTask(store, t.id, fakePersonal(), {
          host,
          policy: mergePolicies({ user: corpToPersonal }),
          approve: async (_ev, brief) => {
            seen = brief;
            return false;
          },
        }),
      ).rejects.toThrow(/nothing was sent/);
      expect(seen).toContain('Informe trimestral');
      expect((await store.loadTask(t.id)).runs).toHaveLength(0);
    });

    it('runs once approved', async () => {
      const t = await createTask(store, { title: 'x', classification: 'corporate', mode: 'read' });
      const r = await runTask(store, t.id, fakePersonal(), {
        host,
        policy: mergePolicies({ user: corpToPersonal }),
        approve: async () => true,
      });
      expect(r.run.state).toBe('completed');
    });

    it('an enforced managed deny cannot be approved over', async () => {
      const t = await createTask(store, { title: 'x', classification: 'corporate', mode: 'read' });
      const deny = { ...approvalRule, effect: 'deny' as const, enforced: true };
      await expect(
        runTask(store, t.id, fakePersonal(), {
          host,
          policy: mergePolicies({ managed: { version: 1, rules: [deny] } }),
          approve: async () => true,
        }),
      ).rejects.toThrow(/cannot be approved/);
    });
  },
);
