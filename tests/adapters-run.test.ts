import { readFileSync } from 'node:fs';
import path from 'node:path';
import { describe, expect, it } from 'vitest';
import { type AgentEvent, decideOutcome } from '../src/adapters/events.js';
import { adapterFor } from '../src/adapters/index.js';
import { parseClaudeLine, parseCodexLine } from '../src/adapters/parsers.js';
import { startRun } from '../src/adapters/runner.js';
import { ProfileSchema } from '../src/domain/index.js';

const fx = (name: string) =>
  readFileSync(path.join(import.meta.dirname, 'fixtures', name), 'utf8')
    .split('\n')
    .filter(Boolean);
const posix = process.platform !== 'win32';
const FAKE = path.join(import.meta.dirname, 'fixtures', 'fake-agent.mjs');

describe('Claude parser with real stream-json (spike 002)', () => {
  it('normalises a successful run', () => {
    const events = fx('claude-stream-ok.jsonl').flatMap(parseClaudeLine);
    const types = events.map((e) => e.type);
    expect(types).toContain('session');
    expect(types).toContain('limits');
    const outcome = decideOutcome(events, false);
    expect(outcome.state).toBe('completed');
    expect(outcome.resultText).toBe('OK');
    expect(outcome.costUsd).toBeGreaterThan(0);
    expect(outcome.limits?.map((w) => w.name)).toEqual(['five_hour', 'seven_day']);
  });

  it('treats the SIGINT stream (exit 0, is_error) as cancelled only if we asked', () => {
    const events = fx('claude-stream-sigint.jsonl').flatMap(parseClaudeLine);
    expect(decideOutcome(events, true).state).toBe('cancelled');
    expect(decideOutcome(events, false).state).toBe('failed');
  });
});

describe('Codex parser with real exec --json (spike 002)', () => {
  it('normalises a successful run', () => {
    const st = { lastMessage: '' };
    const events = fx('codex-exec-ok.jsonl').flatMap((l) => parseCodexLine(l, st));
    const outcome = decideOutcome(events, false);
    expect(outcome).toMatchObject({
      state: 'completed',
      resultText: 'OK',
      sessionId: expect.any(String),
    });
    expect(outcome.inputTokens).toBeGreaterThan(0);
  });

  it('maps a failed turn with a usage limit to blocked', () => {
    const st = { lastMessage: '' };
    const events = [
      '{"type":"thread.started","thread_id":"t"}',
      '{"type":"turn.failed","error":{"message":"You have hit your usage limit"}}',
    ].flatMap((l) => parseCodexLine(l, st));
    expect(decideOutcome(events, false)).toMatchObject({
      state: 'blocked',
      blockedReason: 'rate_limit',
    });
  });
});

describe('outcome rules (FR-017)', () => {
  const session: AgentEvent = { type: 'session', sessionId: 's' };
  it('no final event → uncertain', () => {
    expect(decideOutcome([session], false).state).toBe('uncertain');
  });
  it('auth errors block instead of failing', () => {
    expect(
      decideOutcome(
        [session, { type: 'result', ok: false, text: '', error: 'authentication_failed' }],
        false,
      ),
    ).toMatchObject({ state: 'blocked', blockedReason: 'auth' });
  });
  it('does not misread free text that mentions auth', () => {
    expect(
      decideOutcome(
        [session, { type: 'result', ok: false, text: 'auth module broke', error: 'x' }],
        false,
      ).state,
    ).toBe('failed');
  });
});

describe('adapter commands', () => {
  const base = {
    id: 'p',
    label: 'p',
    auth: { mode: 'subscription' },
    configDir: '/h/.hilvan-profiles/p',
    destination: { launcher: 'native' },
    accepts: ['personal'],
  };
  it('claude: strict MCP, no prompts, mode-based permissions, prompt not in args', () => {
    const p = ProfileSchema.parse({
      ...base,
      app: 'claude-code',
      tools: { mcpConfig: '/h/mcp.json' },
    });
    const cmd = adapterFor('claude-code').runCommand(p, { cwd: '/repo', mode: 'read' }, {});
    expect(cmd.args).toEqual(
      expect.arrayContaining([
        '-p',
        '--strict-mcp-config',
        '--mcp-config',
        '/h/mcp.json',
        'dontAsk',
      ]),
    );
    expect(cmd.env.CLAUDE_CONFIG_DIR).toBe('/h/.hilvan-profiles/p');
    const w = adapterFor('claude-code').runCommand(
      p,
      { cwd: '/repo', mode: 'write', resumeSessionId: 's1' },
      {},
    );
    expect(w.args).toEqual(expect.arrayContaining(['acceptEdits', '--resume', 's1']));
  });
  it('codex: sandbox by mode, prompt from stdin', () => {
    const p = ProfileSchema.parse({ ...base, app: 'codex' });
    const cmd = adapterFor('codex').runCommand(p, { cwd: '/repo', mode: 'read' }, {});
    expect(cmd.args).toEqual(['exec', '--json', '--cd', '/repo', '--sandbox', 'read-only', '-']);
  });
});

describe.runIf(posix)('runner with a fake agent', () => {
  const run = (scenario: string, opts = {}) => {
    const events: AgentEvent[] = [];
    const h = startRun(
      { command: FAKE, args: [], env: { ...process.env, FAKE_SCENARIO: scenario } },
      'hello prompt',
      parseClaudeLine,
      (e) => events.push(e),
      opts,
    );
    return { h, events };
  };

  it('completes and reports usage and limits', async () => {
    const { h, events } = run('success');
    const o = await h.done;
    expect(o.state).toBe('completed');
    expect(o.costUsd).toBeCloseTo(0.0123);
    expect(events.some((e) => e.type === 'message' && e.text.startsWith('got 12 chars'))).toBe(
      true,
    );
  });

  it('blocks on rate limit and on auth errors', async () => {
    expect(await run('limit').h.done).toMatchObject({
      state: 'blocked',
      blockedReason: 'rate_limit',
    });
    expect(await run('auth').h.done).toMatchObject({ state: 'blocked', blockedReason: 'auth' });
  });

  it('a crash without result is uncertain', async () => {
    expect((await run('crash').h.done).state).toBe('uncertain');
  });

  it('cancels with SIGINT', async () => {
    const { h } = run('slow');
    setTimeout(() => h.cancel(), 300);
    expect((await h.done).state).toBe('cancelled');
  });

  it('kills a run whose output exceeds the line limit (SR-009)', async () => {
    const o = await run('huge', { maxLine: 1024 * 1024 }).h.done;
    expect(o.state).toBe('uncertain');
    expect(o.error).toMatch(/output limit/);
  });

  it('times out (SR-025)', async () => {
    const o = await run('slow', { timeoutMs: 300, graceMs: 200 }).h.done;
    expect(o.state).toBe('cancelled');
    expect(o.error).toBe('timeout');
  });

  it('reports a missing executable as failed', async () => {
    const h = startRun(
      { command: '/nonexistent/agent', args: [], env: process.env },
      '',
      parseClaudeLine,
      () => {},
    );
    expect(await h.done).toMatchObject({ state: 'failed' });
  });
});
