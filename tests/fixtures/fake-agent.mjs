#!/usr/bin/env node
// Fake agent CLI emitting Claude-style stream-json, driven by FAKE_SCENARIO.
// success | limit | auth | crash | slow | huge | write
import { writeFileSync } from 'node:fs';

const scenario = process.env.FAKE_SCENARIO ?? 'success';
const out = (o) => process.stdout.write(`${JSON.stringify(o)}\n`);
let prompt = '';
process.stdin.on('data', (d) => (prompt += d));
process.stdin.on('end', async () => {
  out({ type: 'system', subtype: 'init', session_id: 'sess-1', model: 'fake-1', mcp_servers: [] });
  out({
    type: 'assistant',
    message: {
      content: [{ type: 'text', text: `got ${prompt.length} chars \u001b]52;c;ZXZpbA==\u0007` }],
    },
  });
  if (scenario === 'write' || scenario === 'success') {
    out({
      type: 'assistant',
      message: { content: [{ type: 'tool_use', name: 'Edit', input: { file_path: 'hello.txt' } }] },
    });
    if (scenario === 'write') writeFileSync('hello.txt', 'hello from the agent\n');
  }
  if (scenario === 'limit') {
    out({ type: 'system', subtype: 'api_retry', attempt: 1, error: 'rate_limit' });
    out({
      type: 'rate_limit_event',
      rate_limit_info: {
        status: 'rejected',
        unifiedWindows: { five_hour: { utilization: 1, resetsAt: 1 } },
      },
    });
    out({
      type: 'result',
      subtype: 'error_during_execution',
      is_error: true,
      api_error_status: 429,
      session_id: 'sess-1',
    });
    return;
  }
  if (scenario === 'auth') {
    out({
      type: 'result',
      subtype: 'error_during_execution',
      is_error: true,
      api_error_status: 401,
      session_id: 'sess-1',
    });
    return;
  }
  if (scenario === 'crash') process.exit(3);
  if (scenario === 'huge') {
    process.stdout.write(`{"type":"assistant","x":"${'a'.repeat(3 * 1024 * 1024)}"}\n`);
  }
  if (scenario === 'slow') {
    process.on('SIGINT', () => {
      out({
        type: 'result',
        subtype: 'error_during_execution',
        is_error: true,
        session_id: 'sess-1',
      });
      process.exit(0);
    });
    await new Promise((r) => setTimeout(r, 30000));
  }
  out({
    type: 'rate_limit_event',
    rate_limit_info: {
      status: 'allowed',
      unifiedWindows: {
        five_hour: { utilization: 0.4, resetsAt: 1 },
        seven_day: { utilization: 0.2, resetsAt: 2 },
      },
    },
  });
  out({
    type: 'result',
    subtype: 'success',
    is_error: false,
    result: 'Done. token sk-ant-api03-AAAAAAAAAAAAAAAAAAAAAAAAAAAA',
    session_id: 'sess-1',
    total_cost_usd: 0.0123,
    usage: { input_tokens: 10, output_tokens: 5 },
    permission_denials: [],
  });
});
