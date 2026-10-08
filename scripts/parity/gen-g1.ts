// Plan 002 (G1): golden cases produced by the real TypeScript implementation.
// Usage: npx tsx scripts/parity/gen-g1.ts   (rewrites internal/*/testdata/*.golden.json)
// Temporary: removed with the TypeScript code in G5.
import { mkdirSync, writeFileSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import { buildToolCatalog, Criterio, TASK_TYPES, type TaskType } from '../../src/criterio/index.js';
import { fnv1a, tokenize } from '../../src/criterio/features.js';
import { ulid } from '../../src/domain/ids.js';
import { ProfileSchema } from '../../src/domain/index.js';
import { canonical } from '../../src/store/audit.js';

// Deterministic PRNG (mulberry32), independent of Criterio's own.
let seed = 0x6001;
const rnd = () => {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const pick = <T>(xs: readonly T[]): T => xs[Math.floor(rnd() * xs.length)] as T;
const int = (n: number) => Math.floor(rnd() * n);

const WORDS = (
  'refactorizar corregir bug documentar readme implementar endpoint tests cobertura revisar auditar investigar ' +
  'comparar migración seguridad rendimiento arquitectura cliente jira confluence contrato confidencial ticket ' +
  'incidencia sprint wiki slack correo calendario datadog figma login pagos módulo adaptador eventos JSONL ' +
  'Codex Claude WSL Windows tubería caché spike colas sockets hilos interno nómina producción financiero ' +
  'add fix rename extract cleanup feature support build docs guide explain benchmark feasibility error crash'
).split(' ');
const PII = ['ana.perez@example.com', '4111 1111 1111 1111', 'ES9121000418450200051332', ''];
const sentence = (n: number) => Array.from({ length: n }, () => pick(WORDS)).join(' ');
const signals = () => {
  const s: Record<string, unknown> = { title: sentence(1 + int(6)) };
  if (rnd() < 0.6) s.text = `${sentence(int(40))} ${pick(PII)}`.trim();
  if (rnd() < 0.5) s.criteriaCount = int(8);
  if (rnd() < 0.3) s.diff = diff();
  return s;
};
const PATHS = ['src/a.ts', 'src/auth/session.ts', '.env', 'README.md', 'tests/a.test.ts', 'go.mod',
  '.github/workflows/ci.yml', 'docs/guide.md', 'db/migrations/001.sql', 'src\\win\\path.ts', 'spec/x.rb'];
const diff = () => {
  const paths = Array.from({ length: int(6) }, () => pick(PATHS));
  return { filesChanged: paths.length, linesAdded: int(800), linesDeleted: int(800), paths };
};
const RESULTS = ['Listo, tests en verde', 'I could not run the tests', 'No pude compilar', 'Hecho. TODO: docs',
  'All done', 'falló la migración', 'Completed without error', ''];
const STATES = ['completed', 'completed', 'completed', 'failed', 'blocked', 'cancelled', 'uncertain'] as const;
const run = () => {
  const r: Record<string, unknown> = { state: pick(STATES) };
  if (rnd() < 0.8) r.resultText = pick(RESULTS);
  if (rnd() < 0.3) r.permissionDenials = int(4);
  if (rnd() < 0.5) r.criteriaCount = int(5);
  if (rnd() < 0.4) r.diff = diff();
  return r;
};

const profiles = [
  { id: 'claude-corp', tools: { provides: ['jira', 'confluence', 'mcp-datadog-interno'], aliases: { jira: ['caso de soporte'] } } },
  { id: 'claude-personal', tools: { provides: ['github', 'figma'], aliases: {} } },
  { id: 'codex', tools: { provides: ['jira', 'slack'], aliases: { slack: ['hilo del canal'] } } },
].map((p) =>
  ProfileSchema.parse({
    ...p, label: p.id, app: 'claude-code', auth: { mode: 'subscription' },
    destination: { launcher: 'native' }, accepts: ['public'],
  }),
);
const catalog = buildToolCatalog(profiles);
const toolIds = Object.keys(catalog);
const CANDS = [
  [{ id: 'claude-corp', app: 'claude-code' }, { id: 'codex', app: 'codex' }],
  [{ id: 'claude-corp', app: 'claude-code', quotaHeadroom: 0.95 }, { id: 'claude-personal', app: 'claude-code', quotaHeadroom: 0.05 }],
  [{ id: 'claude-personal', app: 'claude-code', quotaHeadroom: 0.3, preference: 2 }, { id: 'codex', app: 'codex', quotaHeadroom: 0.9 }, { id: 'claude-corp', app: 'claude-code', preference: -1 }],
];

const c = new Criterio();
const ops: Array<{ op: string; args: unknown[]; out: unknown }> = [];
const rec = (op: string, args: unknown[], out: unknown) => ops.push({ op, args, out: out ?? null });
for (let i = 0; i < 2500; i++) {
  const r = rnd();
  const type = pick(TASK_TYPES) as TaskType;
  if (r < 0.14) { const s = signals(); rec('taskType', [s], c.taskType(s)); }
  else if (r < 0.22) { const s = signals(); const t = rnd() < 0.7 ? type : undefined; rec('complexity', [s, t ?? ''], c.complexity(s, t)); }
  else if (r < 0.28) { const d = diff(); rec('diffRisk', [d], c.diffRisk(d)); }
  else if (r < 0.35) { const s = signals(); rec('sensitive', [s], c.sensitive(s)); }
  else if (r < 0.40) { const s = signals(); const cur = pick(['public', 'personal', 'corporate', 'confidential']); const d = c.suggestClassification(s, cur); rec('suggestClassification', [s, cur], { ...d, value: d.value ?? '' }); }
  else if (r < 0.47) { const x = run(); rec('meetsCriteria', [x], c.meetsCriteria(x)); }
  else if (r < 0.54) { const s = signals(); rec('requiredTools', [s], c.requiredTools(s, catalog)); }
  else if (r < 0.58) { const cs = pick(CANDS); const sd = int(2 ** 32); rec('rankProfiles', [cs, type, sd], c.rankProfiles(cs, type, sd)); }
  else if (r < 0.70) { const s = signals(); c.learnTaskType(s, type); rec('learnTaskType', [s, type], null); }
  else if (r < 0.74) { const s = signals(); const sc = 2 + int(9); c.learnComplexity(s, type, sc); rec('learnComplexity', [s, type, sc], null); }
  else if (r < 0.80) { const s = signals(); const y = rnd() < 0.5; c.learnSensitivity(s, y); rec('learnSensitivity', [s, y], null); }
  else if (r < 0.86) { const x = run(); const y = rnd() < 0.5; c.learnAcceptance(x, y); rec('learnAcceptance', [x, y], null); }
  else if (r < 0.91) { const p = pick(['claude-corp', 'claude-personal', 'codex']); const y = rnd() < 0.6; c.recordOutcome(p, type, y); rec('recordOutcome', [p, type, y], null); }
  else if (r < 0.98) { const s = signals(); const chosen = toolIds.filter(() => rnd() < 0.3); c.learnRequiredTools(s, catalog, chosen); rec('learnRequiredTools', [s, chosen], null); }
  else rec('stats', [], c.stats());
  if (i % 500 === 499) rec('state', [], JSON.parse(JSON.stringify(c.toJSON())));
}
rec('state', [], JSON.parse(JSON.stringify(c.toJSON())));
rec('stats', [], c.stats());

mkdirSync(new URL('../../internal/criterio/testdata', import.meta.url), { recursive: true });
writeFileSync(
  new URL('../../internal/criterio/testdata/engine.golden.json.gz', import.meta.url),
  gzipSync(JSON.stringify({ profiles: profiles.map((p) => ({ id: p.id, tools: p.tools })), catalog, ops, stateText: JSON.stringify(c.toJSON()) }), { level: 9 }),
);

// Text primitives: tokenize, fnv1a, canonical, ulid.
const texts = ['Refactorizar la documentación', 'İstanbul ǅ ﬁx Kelvin K ß', 'Añadir TESTS 😀 日本語', '', 'a b c'];
for (let i = 0; i < 300; i++) texts.push(sentence(int(12)));
const values: unknown[] = [0, -0, 1e21, 1e-7, 0.1 + 0.2, 'a"b\\c\n <&>', { b: [1, { d: null, c: true }], a: 'x', 'á': 1, A: 2 }];
for (let i = 0; i < 300; i++) values.push({ ts: `2026-10-07T00:00:${String(int(60)).padStart(2, '0')}.000Z`, type: pick(['run.started', 'task.created']), data: { n: rnd() * 10 ** (int(30) - 15), s: sentence(int(5)), list: [int(9), null, rnd() < 0.5] } });
const ulids = Array.from({ length: 50 }, () => {
  const now = int(2 ** 40);
  const rand = Array.from({ length: 10 }, () => int(256));
  return { now, rand, out: ulid(now, Uint8Array.from(rand)) };
});
mkdirSync(new URL('../../internal/jsfmt/testdata', import.meta.url), { recursive: true });
writeFileSync(
  new URL('../../internal/jsfmt/testdata/text.golden.json', import.meta.url),
  JSON.stringify({
    tokens: texts.map((t) => ({ text: t, tokens: tokenize(t), fnv: fnv1a(t) })),
    canonical: values.map((v) => ({ in: v, out: canonical(v) })),
    ulids,
  }),
);
console.log(`ops=${ops.length} texts=${texts.length} values=${values.length}`);

// task.json: key order, defaults and number formatting of JSON.stringify(TaskSchema.parse(x), null, 2).
import('../../src/domain/index.js').then(({ TaskSchema }) => {
  const id = (p: string, n: number) => `${p}_01JABCDEFGHJKMNPQRSTVWXY${String(n).padStart(2, '0')}`;
  const at = '2026-10-07T12:34:56.789Z';
  const inputs: unknown[] = [
    { id: id('tsk', 1), title: 'Mínima <&>', classification: 'personal', mode: 'read', state: 'draft', createdAt: at, updatedAt: at },
    {
      updatedAt: at, createdAt: at, id: id('tsk', 2), title: 'Completa "con" comillas', classification: 'corporate', mode: 'write',
      requires: ['jira'], taskType: 'fix', complexity: 6.5, criteriaCount: 3, profileId: 'claude-corp', baseBranch: 'main',
      state: 'in_review', branch: 'hilvan/x', worktree: '/w/x', handoffs: [id('ho', 3)], reviewedHead: 'abc', approvedProfiles: ['claude-personal'],
      runs: [{ id: id('run', 4), profileId: 'claude-corp', state: 'blocked', blockedReason: 'rate_limit', sessionId: 's',
        usage: { estimated: true, costUsd: 0.000001234, inputTokens: 12000, outputTokens: 34 },
        limits: [{ name: 'five_hour', utilization: 0.97, resetsAt: 1791234567 }],
        diff: { filesChanged: 2, linesAdded: 10, linesDeleted: 1, paths: ['a', 'b'] }, risk: 4.2, meetsCriteria: 0.123456789,
        error: 'límite', summary: 'línea\nnueva', startedAt: at, endedAt: at }],
      reviews: [{ id: id('run', 5), profileId: 'codex', verdict: 'changes_requested', usage: { estimated: true, costUsd: 1e-7 }, at }],
      extraField: 'stripped by zod',
    },
  ];
  writeFileSync(
    new URL('../../internal/domain/testdata/tasks.golden.json', import.meta.url),
    JSON.stringify(inputs.map((x) => ({ in: x, out: `${JSON.stringify(TaskSchema.parse(x), null, 2)}\n` }))),
  );
});
