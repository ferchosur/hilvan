import spawn from 'cross-spawn';
import type { Invocation } from '../launchers/index.js';
import { LineLimitError, LineSplitter } from '../process/lines.js';
import { assertSafeArgs } from '../security/args.js';
import { type AgentEvent, decideOutcome, type RunOutcome } from './events.js';

export interface RunOptions {
  /** SR-025: hard limit for a run; the run is cancelled when it expires. */
  timeoutMs?: number;
  /** Time between SIGINT and SIGTERM when cancelling. */
  graceMs?: number;
  maxLine?: number;
  maxTotal?: number;
}

export interface RunHandle {
  pid: number | undefined;
  cancel(reason?: string): void;
  done: Promise<RunOutcome & { exitCode: number | null; stderrTail: string }>;
}

/**
 * Spawns an agent CLI (no shell), sends the prompt on stdin, parses its JSONL stdout
 * line by line with limits (SR-009) and decides the final state from the events.
 */
export function startRun(
  inv: Invocation,
  prompt: string,
  parse: (line: string) => AgentEvent[],
  onEvent: (e: AgentEvent, rawLine: string) => void,
  opts: RunOptions = {},
): RunHandle {
  assertSafeArgs(inv.args);
  const events: AgentEvent[] = [];
  let cancelRequested = false;
  let limitError: string | undefined;
  let stderrTail = '';

  const child = spawn(inv.command, inv.args, {
    env: inv.env,
    ...(inv.cwd ? { cwd: inv.cwd } : {}),
    stdio: ['pipe', 'pipe', 'pipe'],
    windowsHide: true,
  });

  const kill = (signal: NodeJS.Signals) => {
    try {
      child.kill(signal);
    } catch {
      // already gone
    }
  };
  let graceTimer: NodeJS.Timeout | undefined;
  const cancel = (reason?: string) => {
    if (cancelRequested) return;
    cancelRequested = true;
    if (reason) limitError ??= reason;
    kill('SIGINT');
    graceTimer = setTimeout(() => {
      kill('SIGTERM');
      graceTimer = setTimeout(() => kill('SIGKILL'), opts.graceMs ?? 10_000);
    }, opts.graceMs ?? 10_000);
  };
  const timeout = opts.timeoutMs ? setTimeout(() => cancel('timeout'), opts.timeoutMs) : undefined;

  const splitter = new LineSplitter(
    (line) => {
      for (const e of parse(line)) {
        events.push(e);
        onEvent(e, line);
      }
    },
    {
      ...(opts.maxLine ? { maxLine: opts.maxLine } : {}),
      ...(opts.maxTotal ? { maxTotal: opts.maxTotal } : {}),
    },
  );

  child.stdout?.setEncoding('utf8');
  child.stdout?.on('data', (chunk: string) => {
    if (limitError && !cancelRequested) return;
    try {
      splitter.push(chunk);
    } catch (err) {
      if (err instanceof LineLimitError) {
        limitError = `output limit exceeded (${err.reason})`;
        kill('SIGTERM');
      } else throw err;
    }
  });
  child.stderr?.setEncoding('utf8');
  child.stderr?.on('data', (chunk: string) => {
    stderrTail = (stderrTail + chunk).slice(-4096);
  });
  child.stdin?.on('error', () => {
    // agent exited before reading the prompt; the outcome reflects it
  });
  child.stdin?.end(prompt);

  const done = new Promise<RunOutcome & { exitCode: number | null; stderrTail: string }>(
    (resolve) => {
      let spawnError: string | undefined;
      child.on('error', (err: NodeJS.ErrnoException) => {
        spawnError = err.code === 'ENOENT' ? `${inv.command} not found` : err.message;
      });
      child.on('close', (code) => {
        clearTimeout(timeout);
        clearTimeout(graceTimer);
        try {
          splitter.end();
        } catch {
          // ignore trailing partial line beyond limits
        }
        if (spawnError) {
          resolve({ state: 'failed', error: spawnError, exitCode: code, stderrTail });
          return;
        }
        const outcome = decideOutcome(events, cancelRequested);
        if (limitError && outcome.state === 'uncertain') outcome.error = limitError;
        if (limitError === 'timeout') outcome.error = 'timeout';
        if (!outcome.error && outcome.state !== 'completed' && stderrTail.trim()) {
          outcome.error = stderrTail.trim().split('\n').slice(-1)[0] ?? '';
        }
        resolve({ ...outcome, exitCode: code, stderrTail });
      });
    },
  );

  return { pid: child.pid, cancel, done };
}
