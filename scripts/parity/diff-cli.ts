// Plan 002 (G4): differential test of the CLI. Runs the same scenario with the TypeScript CLI
// (dist/) and the Go CLI, each in its own sandbox (temp git repo, HILVAN_CONFIG_DIR, fake agent),
// and compares stdout, stderr and exit codes after normalising ids, times, hashes and paths.
// Usage: npm run build && go build -o <bin> ./cmd/hilvan && npx tsx scripts/parity/diff-cli.ts <bin>
// Temporary: removed with the TypeScript code in G5.
import { execFileSync, spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const here = (p: string) => new URL(`../../${p}`, import.meta.url).pathname;
const goBin = path.resolve(process.argv[2] ?? 'hilvan-go');
const FAKE = here('tests/fixtures/fake-agent.mjs');

interface Step {
  args: string[];
  scenario?: string;
  /** Compare JSON semantically (key order may differ). */
  json?: boolean;
  /** Thompson sampling: scores and weights are random by design. */
  random?: boolean;
  note?: string;
}

const P = (id: string, ...extra: string[]) => [
  'profile', 'add', '--id', id, '--app', 'claude-code', '--accepts', 'public,personal',
  '--executable', FAKE, '--config-dir', '@TMP@/cfg/' + id, ...extra,
];

const steps: Step[] = [
  { args: ['--version'] },
  { args: ['profile', 'list'] },
  { args: P('claude-a', '--tags', 'personal') },
  { args: [...P('corp', '--routing', 'reserved', '--provides', 'jira,intranet-search', '--alias', 'jira=caso de soporte', '--tags', 'corporate').slice(0, 7), 'public,personal,corporate', ...P('corp', '--routing', 'reserved', '--provides', 'jira,intranet-search', '--alias', 'jira=caso de soporte', '--tags', 'corporate').slice(8)] },
  { args: ['profile', 'add', '--id', 'codex-b', '--app', 'codex', '--accepts', 'public,personal', '--executable', FAKE, '--config-dir', '@TMP@/cfg/codex-b', '--model', 'gpt-x'] },
  { args: P('dup', '--config-dir', '@TMP@/cfg/claude-a'), note: 'shared config dir is rejected' },
  { args: ['profile', 'add', '--id', 'x'], note: 'non-interactive without required options' },
  { args: ['profile', 'add', '--id', 'Bad Id', '--app', 'claude-code', '--accepts', 'public'] },
  { args: ['profile', 'add', '--id', 'y', '--app', 'other', '--accepts', 'public'] },
  { args: ['profile', 'list'] },
  { args: ['profile', 'list', '--json'], json: true },
  { args: ['profile', 'set', 'claude-a', '--preference', '2', '--alias', 'github=pull request'] },
  { args: ['profile', 'set', 'nope'] },
  { args: ['policy', 'show'] },
  { args: ['policy', 'show', '--json'], json: true },
  { args: ['criterio', 'ask', 'Refactorizar el módulo de pagos', '--text', 'Datos de clientes en Jira interno', '--criteria', '3'] },
  { args: ['criterio', 'ask', 'Add support for SSH destinations', '--json'], json: true },
  { args: ['criterio', 'teach', 'Spike de colas', '--type', 'research'] },
  { args: ['criterio', 'teach', 'x', '--type', 'nope'] },
  { args: ['criterio', 'stats'] },
  { args: ['task', 'list'], note: 'outside a project' },
  { args: ['init'] },
  { args: ['task', 'list'] },
  { args: ['task', 'new', 'Add hello file', '--objective', 'Create hello.txt with a greeting', '--criteria', 'hello.txt exists', '--criteria', 'tests pass', '--classification', 'personal'] },
  { args: ['task', 'new', 'Exportar incidencias del sprint en Jira', '--objective', 'Datos de clientes, contrato confidencial', '--mode', 'read', '--classification', 'corporate'] },
  { args: ['task', 'new', 'Corregir bug del login', '--type', 'fix', '--mode', 'read'] },
  { args: ['task', 'new', 'Needs a tool nobody has', '--mode', 'read', '--requires', 'none-tool'] },
  { args: ['task', 'run', '@T4@', '-p', 'claude-a'], note: 'no eligible profile' },
  { args: ['task', 'new', 'x', '--mode', 'delete'] },
  { args: ['task', 'list'] },
  { args: ['task', 'list', '--json'], json: true },
  { args: ['task', 'show', '@T1@'] },
  { args: ['task', 'recommend', '@T2@'], random: true },
  { args: ['task', 'recommend', '@T2@', '--json'], json: true, random: true },
  { args: ['task', 'requires', '@T2@', 'jira'] },
  { args: ['policy', 'check', '@T2@', '-p', 'claude-a'] },
  { args: ['policy', 'check', '@T2@', '-p', 'corp'] },
  { args: ['task', 'run', '@T1@', '-p', 'claude-a'], scenario: 'write' },
  { args: ['task', 'show', '@T1@'] },
  { args: ['task', 'log', '@T1@'] },
  { args: ['task', 'run', '@T1@', '-p', 'codex-b'], scenario: 'success', note: 'no silent profile switch' },
  { args: ['review', '@T1@', '--stat'] },
  { args: ['accept', '@T1@'] },
  { args: ['review', '@T1@'] },
  { args: ['accept', '@T1@'] },
  { args: ['task', 'run', '@T3@', '-p', 'claude-a'], scenario: 'limit' },
  { args: ['handoff', '@T3@', '--to', 'codex-b', '--print'] },
  { args: ['handoff', '@T3@', '--to', 'codex-b'], note: 'not a terminal: cancelled' },
  { args: ['handoff', '@T3@', '--to', 'codex-b', '--approve'] },
  { args: ['task', 'run', '@T3@', '-p', 'codex-b'], scenario: 'success' },
  { args: ['review', '@T3@', '--by', 'claude-a'], scenario: 'review' },
  { args: ['review', '@T3@', '--by', 'codex-b'], scenario: 'review' },
  { args: ['stats'] },
  { args: ['stats', '--json'], json: true },
  { args: ['task', 'discard', '@T3@'] },
  { args: ['task', 'discard', '@T3@', '--yes'] },
  { args: ['task', 'cancel', '@T2@'] },
  { args: ['audit', 'verify'] },
  { args: ['criterio', 'stats'] },
  { args: ['profile', 'remove', 'corp', '--yes'] },
  { args: ['profile', 'list'] },
];

interface Result {
  code: number | null;
  stdout: string;
  stderr: string;
}

function runScenario(cli: string[]): Result[] {
  const tmp = mkdtempSync(path.join(os.tmpdir(), 'hilvan-diff-'));
  const repo = path.join(tmp, 'repo');
  execFileSync('git', ['init', '-q', '-b', 'main', repo]);
  execFileSync('git', ['-C', repo, '-c', 'user.name=t', '-c', 'user.email=t@t', 'commit', '-q', '--allow-empty', '-m', 'init']);
  const tasks: string[] = [];
  const out: Result[] = [];
  for (const step of steps) {
    const args = step.args.map((a) =>
      a.replaceAll('@TMP@', tmp).replace(/@T(\d)@/, (_, n) => tasks[Number(n) - 1]?.slice(-6) ?? 'none'),
    );
    const r = spawnSync(cli[0] as string, [...cli.slice(1), ...args], {
      cwd: repo,
      encoding: 'utf8',
      env: {
        PATH: process.env.PATH,
        HOME: tmp,
        HILVAN_CONFIG_DIR: path.join(tmp, 'config'),
        FAKE_SCENARIO: step.scenario ?? 'success',
        GIT_CONFIG_GLOBAL: '/dev/null',
        GIT_CONFIG_NOSYSTEM: '1',
        LANG: 'C.UTF-8',
      },
      input: '',
    });
    const created = /✔ task (\w{6}) created/.exec(r.stdout);
    if (created?.[1]) {
      const full = /tsk_[0-9A-Z]{20}/.exec(r.stdout)?.[0];
      tasks.push(full ?? created[1]);
    }
    out.push({ code: r.status, stdout: r.stdout, stderr: r.stderr });
  }
  const results = out.map((r) => normalise(r, tmp));
  rmSync(tmp, { recursive: true, force: true });
  return results;
}

function normalise(r: Result, tmp: string): Result {
  const ids = new Map<string, string>();
  const id = (prefix: string) => (m: string) => {
    if (!ids.has(m)) ids.set(m, `${prefix}${ids.size}`);
    return ids.get(m) as string;
  };
  const norm = (s: string) =>
    s
      .replaceAll(tmp, '$TMP')
      .replace(/(tsk|run|ho)_[0-9A-Z]{26}/g, id('ID'))
      .replace(/\b[0-9A-Z]{6}\b/g, 'SHORT')
      .replace(/\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2}(\.\d+)?Z)?/g, 'TIME')
      .replace(/\b[0-9a-f]{64}\b/g, 'SHA256')
      .replace(/sha256 [0-9a-f]{12}/g, 'sha256 HASH')
      .replace(/commit [0-9a-f]{7,12}\b/g, 'commit HASH')
      .replace(/index [0-9a-f]+\.\.[0-9a-f]+/g, 'index HASH')
      .replace(/"pid":\s*\d+/g, '"pid": N');
  return { code: r.code, stdout: norm(r.stdout), stderr: norm(r.stderr) };
}

function sameJSON(a: string, b: string): boolean {
  const canon = (v: unknown): unknown =>
    Array.isArray(v)
      ? v.map(canon)
      : v && typeof v === 'object'
        ? Object.fromEntries(Object.entries(v).sort(([x], [y]) => (x < y ? -1 : 1)).map(([k, x]) => [k, canon(x)]))
        : typeof v === 'number'
          ? Number(v.toPrecision(12))
          : v;
  try {
    return JSON.stringify(canon(JSON.parse(a))) === JSON.stringify(canon(JSON.parse(b)));
  } catch {
    return a === b;
  }
}

function unrandom(r: Result, json: boolean): Result {
  if (!json) return { ...r, stdout: r.stdout.replace(/score \d+\.\d+/g, 'score X') };
  return { ...r, stdout: r.stdout.replace(/"(score|weight)": [0-9.e-]+/g, '"$1": 0') };
}

const ts = runScenario(['node', here('dist/cli/main.js')]);
const go = runScenario([goBin]);
let failures = 0;
steps.forEach((step, i) => {
  const a = step.random ? unrandom(ts[i] as Result, !!step.json) : (ts[i] as Result);
  const b = step.random ? unrandom(go[i] as Result, !!step.json) : (go[i] as Result);
  const same =
    a.code === b.code &&
    (step.json ? sameJSON(a.stdout, b.stdout) : a.stdout === b.stdout) &&
    a.stderr === b.stderr;
  if (!same) {
    failures++;
    console.log(`\n✖ step ${i}: hilvan ${step.args.join(' ')}${step.note ? `  (${step.note})` : ''}`);
    console.log(`  exit ts=${a.code} go=${b.code}`);
    if (a.stdout !== b.stdout) console.log(`  --- ts stdout\n${a.stdout}\n  --- go stdout\n${b.stdout}`);
    if (a.stderr !== b.stderr) console.log(`  --- ts stderr\n${a.stderr}\n  --- go stderr\n${b.stderr}`);
  }
});
if (process.env.DUMP) {
  steps.forEach((step, i) => {
    const b = go[i] as Result;
    console.log(`\n=== ${i}: hilvan ${step.args.join(' ')} → exit ${b.code}\n${b.stdout}${b.stderr ? `[stderr] ${b.stderr}` : ''}`);
  });
}
console.log(`\n${steps.length - failures}/${steps.length} steps identical`);
process.exitCode = failures ? 1 : 0;
