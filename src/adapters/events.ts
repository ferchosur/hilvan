import type { BlockedReason, LimitWindow } from '../domain/index.js';

/** Normalised agent events (docs/design/contrato-adaptador.md §2). */
export type AgentEvent =
  | { type: 'session'; sessionId: string; model?: string; mcpServers?: string[] }
  | { type: 'message'; text: string }
  | { type: 'tool'; name: string; status: 'started' | 'completed' | 'failed'; summary?: string }
  | { type: 'retry'; attempt: number; error: string }
  | { type: 'limits'; status: string; windows: LimitWindow[] }
  | { type: 'usage'; costUsd?: number; inputTokens?: number; outputTokens?: number }
  | {
      type: 'result';
      ok: boolean;
      text: string;
      sessionId?: string;
      stopReason?: string;
      permissionDenials?: number;
      error?: string;
    }
  | { type: 'raw'; line: string };

export type RunState = 'completed' | 'failed' | 'blocked' | 'cancelled' | 'uncertain';

export interface RunOutcome {
  state: RunState;
  blockedReason?: BlockedReason;
  sessionId?: string;
  resultText?: string;
  permissionDenials?: number;
  costUsd?: number;
  inputTokens?: number;
  outputTokens?: number;
  limits?: LimitWindow[];
  error?: string;
}

const AUTH = new Set(['authentication_failed', 'oauth_org_not_allowed', 'account_on_hold']);

function classifyError(err: string): BlockedReason | null {
  if (err === 'rate_limit' || /rate.?limit|usage limit|quota/i.test(err)) return 'rate_limit';
  if (AUTH.has(err) || /auth|login|unauthori[sz]ed|401|403/i.test(err)) return 'auth';
  if (err === 'billing_error' || /billing|credit/i.test(err)) return 'billing';
  return null;
}

/**
 * Final state from the events, never from the exit code alone (spike 002: Claude exits 0
 * after SIGINT). FR-017: limits/auth/billing → blocked; no final event → uncertain.
 */
export function decideOutcome(events: readonly AgentEvent[], cancelRequested: boolean): RunOutcome {
  const out: RunOutcome = { state: 'uncertain' };
  let lastRetry: string | undefined;
  let result: Extract<AgentEvent, { type: 'result' }> | undefined;
  for (const e of events) {
    switch (e.type) {
      case 'session':
        out.sessionId = e.sessionId;
        break;
      case 'retry':
        lastRetry = e.error;
        break;
      case 'limits':
        out.limits = e.windows;
        if (e.status !== 'allowed' && e.status !== 'allowed_warning') lastRetry ??= 'rate_limit';
        break;
      case 'usage':
        if (e.costUsd !== undefined) out.costUsd = (out.costUsd ?? 0) + e.costUsd;
        if (e.inputTokens !== undefined) out.inputTokens = (out.inputTokens ?? 0) + e.inputTokens;
        if (e.outputTokens !== undefined)
          out.outputTokens = (out.outputTokens ?? 0) + e.outputTokens;
        break;
      case 'result':
        result = e;
        break;
    }
  }
  if (result?.sessionId) out.sessionId = result.sessionId;
  if (result?.permissionDenials) out.permissionDenials = result.permissionDenials;
  if (!result) {
    if (cancelRequested) return { ...out, state: 'cancelled' };
    const reason = lastRetry ? classifyError(lastRetry) : null;
    return reason ? { ...out, state: 'blocked', blockedReason: reason } : out;
  }
  out.resultText = result.text;
  if (result.ok) return { ...out, state: 'completed' };
  if (cancelRequested) return { ...out, state: 'cancelled' };
  // Classify from structured error fields only; free result text may mention 'auth' innocently.
  const reason = classifyError(result.error ?? lastRetry ?? '');
  if (reason)
    return { ...out, state: 'blocked', blockedReason: reason, error: result.error ?? result.text };
  return { ...out, state: 'failed', error: result.error ?? result.text };
}
