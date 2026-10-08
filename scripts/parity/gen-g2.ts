// Plan 002 (G2): golden cases produced by the real TypeScript adapters and audit log.
// Usage: npx tsx scripts/parity/gen-g2.ts   (rewrites internal/{adapters,store}/testdata)
// Temporary: removed with the TypeScript code in G5.
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { type AgentEvent, decideOutcome } from '../../src/adapters/events.js';
import { adapterFor } from '../../src/adapters/index.js';
import { parseClaudeLine, parseCodexLine } from '../../src/adapters/parsers.js';
import { ProfileSchema } from '../../src/domain/index.js';
import { appendAudit } from '../../src/store/audit.js';

const here = (p: string) => new URL(`../../${p}`, import.meta.url).pathname;
mkdirSync(here('internal/adapters/testdata'), { recursive: true });
mkdirSync(here('internal/store/testdata'), { recursive: true });
for (const f of ['claude-stream-ok.jsonl', 'claude-stream-sigint.jsonl', 'codex-exec-ok.jsonl']) {
  copyFileSync(here(`tests/fixtures/${f}`), here(`internal/adapters/testdata/${f}`));
}
const fixture = (f: string) => readFileSync(here(`tests/fixtures/${f}`), 'utf8').split('\n').filter(Boolean);

const j = (o: unknown) => JSON.stringify(o);
const claudeLines = [
  ...fixture('claude-stream-ok.jsonl'),
  ...fixture('claude-stream-sigint.jsonl'),
  'not json at all',
  '[1,2]',
  j({ type: 'system', subtype: 'init', session_id: 's', mcp_servers: [{ name: 'jira' }, { name: '' }, 'x', { name: 3 }] }),
  j({ type: 'system', subtype: 'init' }),
  j({ type: 'system', subtype: 'api_retry', attempt: 2, error: 'rate_limit' }),
  j({ type: 'system', subtype: 'api_retry' }),
  j({ type: 'system', subtype: 'other' }),
  j({ type: 'assistant', message: { content: [{ type: 'text', text: '' }, { type: 'text', text: 'hola <&>' }, null,
    { type: 'tool_use', name: 'Bash', input: { command: 'ls -la' } }, { type: 'tool_use', input: { file_path: 'a.ts' } },
    { type: 'tool_use', name: 'Grep', input: { command: '', pattern: 'x' } }, { type: 'tool_use', name: 'T' }] } }),
  j({ type: 'assistant', message: {} }),
  j({ type: 'user', message: { content: [{ type: 'tool_result', is_error: true }, { type: 'tool_result' }, { type: 'text' }, { type: 'tool_result', is_error: 'yes' }] } }),
  j({ type: 'rate_limit_event', rate_limit_info: { status: 'rejected', unifiedWindows: { seven_day: { utilization: 1.4, resetsAt: 9 }, five_hour: { utilization: -0.1 }, bad: { utilization: 'x' }, z: {} } } }),
  j({ type: 'rate_limit_event' }),
  j({ type: 'result', is_error: true, api_error_status: 429, session_id: 's2', permission_denials: [1, 2] }),
  j({ type: 'result', is_error: true, api_error_status: 401 }),
  j({ type: 'result', is_error: true, api_error_status: 402, subtype: 'x' }),
  j({ type: 'result', is_error: true, subtype: 'error_max_turns', stop_reason: 'max_turns' }),
  j({ type: 'result', is_error: true }),
  j({ type: 'result', is_error: false, result: 'ok', total_cost_usd: 0.5, usage: { input_tokens: 3, cache_read_input_tokens: 4, cache_creation_input_tokens: 5, output_tokens: 6 } }),
  j({ type: 'result', usage: {} }),
  j({ type: 'unknown' }),
];
const codexSeqs = [
  fixture('codex-exec-ok.jsonl'),
  [
    j({ type: 'thread.started', thread_id: 't1' }),
    j({ type: 'item.started', item: { type: 'agent_message', text: 'x' } }),
    j({ type: 'item.completed', item: { type: 'agent_message', text: 'first' } }),
    j({ type: 'item.started', item: { type: 'command_execution', command: 'npm test' } }),
    j({ type: 'item.completed', item: { type: 'command_execution', command: 'npm test', status: 'failed' } }),
    j({ type: 'item.completed', item: { type: 'web_search', query: 'q' } }),
    j({ type: 'item.completed', item: { type: 'mcp_tool_call', tool: 'jira' } }),
    j({ type: 'item.completed', item: { type: 'file_change' } }),
    j({ type: 'item.completed', item: { type: 'reasoning' } }),
    j({ type: 'error', message: 'reconnecting' }),
    j({ type: 'error' }),
    j({ type: 'item.completed', item: { type: 'agent_message', text: 'last' } }),
    j({ type: 'turn.completed', usage: { input_tokens: 10, output_tokens: 2, reasoning_output_tokens: 3 } }),
  ],
  [j({ type: 'thread.started', thread_id: 't2' }), j({ type: 'turn.failed', error: { message: 'You have hit your usage limit' } })],
  [j({ type: 'turn.failed' }), 'garbage', j({ type: 'turn.completed' })],
];

const parsedClaude = claudeLines.map((line) => ({ line, events: parseClaudeLine(line) }));
const parsedCodex = codexSeqs.map((seq) => {
  const st = { lastMessage: '' };
  return seq.map((line) => ({ line, events: parseCodexLine(line, st) }));
});

const seqs: AgentEvent[][] = [
  parsedClaude.slice(0, fixture('claude-stream-ok.jsonl').length).flatMap((x) => x.events),
  fixture('claude-stream-sigint.jsonl').flatMap(parseClaudeLine),
  ...parsedCodex.map((s) => s.flatMap((x) => x.events)),
  [],
  [{ type: 'session', sessionId: '' }],
  [{ type: 'retry', attempt: 1, error: 'rate_limit' }],
  [{ type: 'retry', attempt: 1, error: 'login required' }],
  [{ type: 'retry', attempt: 1, error: 'nothing' }],
  [{ type: 'limits', status: 'rejected', windows: [] }],
  [{ type: 'retry', attempt: 1, error: 'credit balance' }, { type: 'limits', status: 'allowed_warning', windows: [{ name: 'w', utilization: 0.9 }] }],
  [{ type: 'session', sessionId: 's' }, { type: 'result', ok: false, text: 'auth module broke', error: 'x' }],
  [{ type: 'session', sessionId: 's' }, { type: 'result', ok: false, text: 'boom' }],
  [{ type: 'retry', attempt: 1, error: 'quota exceeded' }, { type: 'result', ok: false, text: 'boom' }],
  [{ type: 'result', ok: false, text: 't', error: '' }],
  [{ type: 'result', ok: false, text: '', error: 'authentication_failed', sessionId: 'z', permissionDenials: 2 }],
  [{ type: 'usage', costUsd: 0.1 }, { type: 'usage', inputTokens: 5 }, { type: 'usage', costUsd: 0.2, outputTokens: 1 }, { type: 'result', ok: true, text: '' }],
];
const outcomes = seqs.flatMap((events) => [false, true].map((cancel) => ({ events, cancel, out: decideOutcome(events, cancel) })));

const claude = adapterFor('claude-code');
const codex = adapterFor('codex');
const auth = [
  ['claude', j({ loggedIn: true, authMethod: 'claude.ai', email: 'a@example.com', orgName: "a@example.com's Organization" }), '', 0],
  ['claude', j({ loggedIn: true, authMethod: 'claude.ai', email: 'a@example.com', orgName: 'Acme Corp' }), '', 0],
  ['claude', j({ loggedIn: true, orgName: 'Only Org' }), '', 0],
  ['claude', j({ loggedIn: false, authMethod: 'none' }), '', 1],
  ['claude', 'oops', 'boom\nsecond', 1],
  ['claude', 'first line\r\nsecond', '', 1],
  ['claude', '', '', 2],
  ['claude', '5', '', 0],
  ['codex', 'Logged in using ChatGPT\n', '', 0],
  ['codex', 'Logged in using an API key - sk-***\n', '', 0],
  ['codex', 'Not logged in\n', '', 1],
  ['codex', '', 'Error loading configuration: CODEX_HOME points to "x"', 1],
  ['codex', '', '', 7],
] as const;
const authOut = auth.map(([app, out, err, code]) => ({ app, stdout: out, stderr: err, code, status: adapterFor(app === 'claude' ? 'claude-code' : 'codex').parseAuthStatus(out, err, code) }));

const tools = [
  ['claude', 'Checking MCP server health...\n\njira: https://mcp.example/jira?token=abc - ✔ Connected\nslack: npx slack-mcp - ✗ Failed to connect\nwiki: node w.js - ⏸ Pending\nweird: x - ? Unknown\r\nnot a tool line\n'],
  ['codex', j([{ name: 'jira', enabled: true }, { name: 'off', enabled: false }, { enabled: true }, { name: 3 }])],
  ['codex', 'not json'],
] as const;
const toolsOut = tools.map(([app, out]) => ({ app, stdout: out, tools: adapterFor(app === 'claude' ? 'claude-code' : 'codex').parseToolList(out) }));

const base = { id: 'p', label: 'p', auth: { mode: 'subscription' }, configDir: '/h/.hilvan-profiles/p', destination: { launcher: 'native' }, accepts: ['personal'] };
const profiles = [
  { ...base, app: 'claude-code' },
  { ...base, app: 'claude-code', model: 'opus', tools: { mcp: 'profile', mcpConfig: '/h/mcp.json' } },
  { ...base, app: 'claude-code', configDir: null, tools: { mcp: 'none' }, auth: { mode: 'api-key', apiKeyEnv: 'WORK_KEY' } },
  { ...base, app: 'codex' },
  { ...base, app: 'codex', model: 'gpt-x', configDir: null, auth: { mode: 'api-key', apiKeyEnv: 'WORK_KEY' } },
].map((p) => ProfileSchema.parse(p));
const specs = [
  { cwd: '/repo', mode: 'read' as const },
  { cwd: '/repo', mode: 'write' as const, resumeSessionId: 's1', model: 'sonnet' },
];
const hostEnv = { WORK_KEY: 'secret-value', ANTHROPIC_API_KEY: 'stray' };
const commands = profiles.map((p) => {
  const a = adapterFor(p.app);
  return {
    profile: p,
    env: a.profileEnv(p, hostEnv),
    login: a.loginCommand(p, hostEnv),
    status: a.authStatusCommand(p, hostEnv),
    tools: a.listToolsCommand(p, hostEnv),
    runs: specs.map((spec) => ({ spec, cmd: a.runCommand(p, spec, hostEnv) })),
  };
});

writeFileSync(
  here('internal/adapters/testdata/adapters.golden.json'),
  JSON.stringify({ claude: parsedClaude, codex: parsedCodex, outcomes, auth: authOut, tools: toolsOut, commands, hostEnv }),
);

// Audit chain written by TypeScript: Go must verify it and compute the same hashes.
const dir = mkdtempSync(path.join(os.tmpdir(), 'hilvan-g2-'));
const file = path.join(dir, 'events.jsonl');
const events = [
  { ts: '2026-10-07T00:00:00.000Z', type: 'task.created', taskId: 'tsk_01JABCDEFGHJKMNPQRSTVWXYZ0', data: { classification: 'corporate', title: 'Ñandú <&> "q"' } },
  { ts: '2026-10-07T00:00:01.000Z', type: 'run.started', taskId: 'tsk_01JABCDEFGHJKMNPQRSTVWXYZ0', runId: 'run_01JABCDEFGHJKMNPQRSTVWXYZ1', profileId: 'claude-corp', data: { destination: 'native', mode: 'write' } },
  { ts: '2026-10-07T00:00:02.000Z', type: 'run.finished', data: { costUsd: 0.000001234, tokens: 12000, ratio: 0.1 + 0.2, big: 1e21, list: [1, 'a', true, null, { b: 2, a: 1 }], ' key': 'x' } },
  { ts: '2026-10-07T00:00:03.000Z', type: 'handoff.approved', data: { sha256: 'ab'.repeat(32), emoji: '😀', zz: { y: [], x: {} } } },
  { ts: '2026-10-07T00:00:04.000Z', type: 'no.data' },
];
(async () => {
  for (const e of events) await appendAudit(file, e);
  copyFileSync(file, here('internal/store/testdata/events.ts.jsonl'));
  rmSync(dir, { recursive: true, force: true });
  console.log(`claude=${claudeLines.length} codexSeqs=${codexSeqs.length} outcomes=${outcomes.length} audit=${events.length}`);
})();
