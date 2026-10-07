import type { AgentEvent } from './events.js';

/** Parsers from each CLI's JSONL to normalised events (pure; tested with real fixtures). */

type Json = Record<string, unknown>;
const obj = (v: unknown): Json | undefined =>
  v && typeof v === 'object' && !Array.isArray(v) ? (v as Json) : undefined;
const str = (v: unknown): string | undefined => (typeof v === 'string' ? v : undefined);
const num = (v: unknown): number | undefined =>
  typeof v === 'number' && Number.isFinite(v) ? v : undefined;

function parseJson(line: string): Json | undefined {
  try {
    return obj(JSON.parse(line));
  } catch {
    return undefined;
  }
}

export function parseClaudeLine(line: string): AgentEvent[] {
  const e = parseJson(line);
  if (!e) return [{ type: 'raw', line }];
  switch (e.type) {
    case 'system': {
      if (e.subtype === 'init') {
        const servers = Array.isArray(e.mcp_servers)
          ? e.mcp_servers.map((s) => str(obj(s)?.name)).filter((s): s is string => !!s)
          : [];
        return [
          {
            type: 'session',
            sessionId: str(e.session_id) ?? '',
            ...(str(e.model) ? { model: str(e.model) as string } : {}),
            mcpServers: servers,
          },
        ];
      }
      if (e.subtype === 'api_retry') {
        return [{ type: 'retry', attempt: num(e.attempt) ?? 0, error: str(e.error) ?? 'unknown' }];
      }
      return [];
    }
    case 'assistant': {
      const content = obj(e.message)?.content;
      if (!Array.isArray(content)) return [];
      const out: AgentEvent[] = [];
      for (const c of content.map(obj)) {
        if (c?.type === 'text' && str(c.text))
          out.push({ type: 'message', text: str(c.text) as string });
        if (c?.type === 'tool_use') {
          const input = obj(c.input);
          const summary = str(input?.command) ?? str(input?.file_path) ?? str(input?.pattern);
          out.push({
            type: 'tool',
            name: str(c.name) ?? 'tool',
            status: 'started',
            ...(summary ? { summary } : {}),
          });
        }
      }
      return out;
    }
    case 'user': {
      const content = obj(e.message)?.content;
      if (!Array.isArray(content)) return [];
      return content
        .map(obj)
        .filter((c) => c?.type === 'tool_result')
        .map((c) => ({
          type: 'tool' as const,
          name: 'result',
          status: c?.is_error ? ('failed' as const) : ('completed' as const),
        }));
    }
    case 'rate_limit_event': {
      const info = obj(e.rate_limit_info);
      const windows = obj(info?.unifiedWindows) ?? {};
      return [
        {
          type: 'limits',
          status: str(info?.status) ?? 'unknown',
          windows: Object.entries(windows)
            .map(([name, w]) => ({
              name,
              utilization: num(obj(w)?.utilization),
              resetsAt: num(obj(w)?.resetsAt),
            }))
            .filter(
              (w): w is { name: string; utilization: number; resetsAt: number | undefined } =>
                w.utilization !== undefined,
            )
            .map((w) => ({
              name: w.name,
              utilization: Math.min(1, Math.max(0, w.utilization)),
              ...(w.resetsAt !== undefined ? { resetsAt: w.resetsAt } : {}),
            })),
        },
      ];
    }
    case 'result': {
      const usage = obj(e.usage);
      const denials = Array.isArray(e.permission_denials) ? e.permission_denials.length : 0;
      const isError = e.is_error === true;
      const status = num(e.api_error_status);
      const errorKind =
        status === 429
          ? 'rate_limit'
          : status === 401 || status === 403
            ? 'authentication_failed'
            : status === 402
              ? 'billing_error'
              : (str(e.subtype) ?? 'error');
      const events: AgentEvent[] = [
        {
          type: 'usage',
          ...(num(e.total_cost_usd) !== undefined
            ? { costUsd: num(e.total_cost_usd) as number }
            : {}),
          ...(usage
            ? {
                inputTokens:
                  (num(usage.input_tokens) ?? 0) +
                  (num(usage.cache_read_input_tokens) ?? 0) +
                  (num(usage.cache_creation_input_tokens) ?? 0),
                outputTokens: num(usage.output_tokens) ?? 0,
              }
            : {}),
        },
        {
          type: 'result',
          ok: !isError,
          text: str(e.result) ?? '',
          ...(str(e.session_id) ? { sessionId: str(e.session_id) as string } : {}),
          ...(str(e.stop_reason) ? { stopReason: str(e.stop_reason) as string } : {}),
          ...(denials ? { permissionDenials: denials } : {}),
          ...(isError ? { error: errorKind } : {}),
        },
      ];
      return events;
    }
    default:
      return [];
  }
}

/** Codex keeps the last agent message to use as the result text of `turn.completed`. */
export interface CodexParserState {
  lastMessage: string;
}

export function parseCodexLine(line: string, st: CodexParserState): AgentEvent[] {
  const e = parseJson(line);
  if (!e) return [{ type: 'raw', line }];
  switch (e.type) {
    case 'thread.started':
      return [{ type: 'session', sessionId: str(e.thread_id) ?? '' }];
    case 'item.started':
    case 'item.completed': {
      const item = obj(e.item);
      const kind = str(item?.type);
      if (kind === 'agent_message') {
        const text = str(item?.text) ?? '';
        if (e.type === 'item.completed') {
          st.lastMessage = text;
          return [{ type: 'message', text }];
        }
        return [];
      }
      if (
        kind === 'command_execution' ||
        kind === 'file_change' ||
        kind === 'mcp_tool_call' ||
        kind === 'web_search'
      ) {
        const summary = str(item?.command) ?? str(item?.query) ?? str(item?.tool);
        const failed = str(item?.status) === 'failed';
        return [
          {
            type: 'tool',
            name: kind,
            status: e.type === 'item.started' ? 'started' : failed ? 'failed' : 'completed',
            ...(summary ? { summary } : {}),
          },
        ];
      }
      return [];
    }
    case 'turn.completed': {
      const u = obj(e.usage);
      return [
        {
          type: 'usage',
          inputTokens: num(u?.input_tokens) ?? 0,
          outputTokens: (num(u?.output_tokens) ?? 0) + (num(u?.reasoning_output_tokens) ?? 0),
        },
        { type: 'result', ok: true, text: st.lastMessage },
      ];
    }
    case 'turn.failed': {
      const msg = str(obj(e.error)?.message) ?? 'turn failed';
      return [{ type: 'result', ok: false, text: st.lastMessage, error: msg }];
    }
    case 'error':
      return [{ type: 'retry', attempt: 0, error: str(e.message) ?? 'error' }];
    default:
      return [];
  }
}
