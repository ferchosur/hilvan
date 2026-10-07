import type { App } from '../domain/index.js';
import { claudeCodeAdapter } from './claude-code.js';
import { codexAdapter } from './codex.js';
import type { AgentAdapter } from './types.js';

export type { AgentEvent, RunOutcome, RunState } from './events.js';
export type { AgentAdapter, AuthStatus, RunSpec } from './types.js';

const ADAPTERS: Readonly<Record<App, AgentAdapter>> = {
  'claude-code': claudeCodeAdapter,
  codex: codexAdapter,
};

export function adapterFor(app: App): AgentAdapter {
  return ADAPTERS[app];
}
