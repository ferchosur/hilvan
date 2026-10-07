import { appendFile, mkdir } from 'node:fs/promises';
import path from 'node:path';
import { type AgentEvent, adapterFor, type RunOutcome } from '../adapters/index.js';
import { type RunHandle, type RunOptions, startRun } from '../adapters/runner.js';
import type { Criterio, Decision, TaskType } from '../criterio/index.js';
import { newRunId, newTaskId } from '../domain/ids.js';
import {
  canTransition,
  type Profile,
  type RunSummary,
  type Task,
  type TaskState,
} from '../domain/index.js';
import type { EffectivePolicy } from '../domain/policy.js';
import {
  commitWorktree,
  createWorktree,
  defaultGit,
  diffSummary,
  type Git,
  removeWorktree,
} from '../gitops/index.js';
import { buildInvocation, type HostContext } from '../launchers/index.js';
import { type Evaluation, evaluate, mergePolicies } from '../policy/evaluate.js';
import { runCommand } from '../process/run.js';
import { redactSecrets } from '../security/secrets.js';
import { type ProjectStore, processAlive } from '../store/project.js';

/** Use cases for tasks and runs (M2). Presentation lives in `cli/task.ts`. */

export class TaskError extends Error {
  override name = 'TaskError';
}

export interface Brief {
  title: string;
  objective?: string;
  criteria?: string[];
  constraints?: string[];
  classification: string;
  mode: 'write' | 'read';
  requires?: string[];
  taskType?: TaskType;
  complexity?: number;
}

export function renderBrief(b: Brief): string {
  const sec = (title: string, items?: string[]) =>
    items?.length ? `\n## ${title}\n\n${items.map((i) => `- ${i}`).join('\n')}\n` : '';
  return (
    `# ${b.title}\n\n` +
    `- Clasificación: ${b.classification}\n- Modo: ${b.mode}\n` +
    (b.objective ? `\n## Objetivo\n\n${b.objective}\n` : '') +
    sec('Criterios de aceptación', b.criteria) +
    sec('Restricciones', b.constraints)
  );
}

export interface Suggestions {
  taskType: Decision<TaskType>;
  complexity: Decision<number>;
  classification: Decision<string | null>;
}

/** Criterio suggestions for a brief (FR-019, FR-020). Pure apart from the engine state. */
export function suggest(criterio: Criterio, b: Brief): Suggestions {
  const s = {
    title: b.title,
    text: [b.objective, ...(b.criteria ?? []), ...(b.constraints ?? [])].filter(Boolean).join('\n'),
    criteriaCount: b.criteria?.length ?? 0,
  };
  const taskType = criterio.taskType(s);
  return {
    taskType,
    complexity: criterio.complexity(s, b.taskType ?? taskType.value),
    classification: criterio.suggestClassification(s, b.classification),
  };
}

export async function createTask(
  store: ProjectStore,
  b: Brief,
  criterio?: Criterio,
): Promise<Task> {
  const now = new Date().toISOString();
  const project = await store.project();
  const task: Task = {
    id: newTaskId(),
    title: b.title,
    classification: b.classification,
    mode: b.mode,
    requires: b.requires ?? [],
    ...(b.taskType ? { taskType: b.taskType } : {}),
    ...(b.complexity ? { complexity: b.complexity } : {}),
    criteriaCount: b.criteria?.length ?? 0,
    profileId: null,
    baseBranch: project.baseBranch,
    state: 'ready',
    branch: null,
    worktree: null,
    runs: [],
    handoffs: [],
    approvedProfiles: [],
    reviews: [],
    reviewedHead: null,
    createdAt: now,
    updatedAt: now,
  };
  // SR-013: secrets typed into a brief are redacted before they are stored or sent.
  const brief = redactSecrets(renderBrief(b)).text;
  await store.saveTask(task, brief);
  if (criterio && b.taskType) {
    criterio.learnTaskType({ title: b.title, text: b.objective ?? '' }, b.taskType);
  }
  await store.audit({
    type: 'task.created',
    taskId: task.id,
    data: { classification: task.classification, mode: task.mode, taskType: task.taskType },
  });
  return task;
}

/** Minimal eligibility for M2 (full layered policy arrives in M3): classification and tools. */
export function eligibility(task: Task, profile: Profile): string[] {
  const reasons: string[] = [];
  if (!profile.accepts.includes(task.classification)) {
    reasons.push(`profile ${profile.id} does not accept "${task.classification}" data`);
  }
  const missing = task.requires.filter((r) => !profile.tools.provides.includes(r));
  if (missing.length)
    reasons.push(`profile ${profile.id} lacks required tools: ${missing.join(', ')}`);
  return reasons;
}

export function buildPrompt(task: Task, brief: string, cwd: string, handoff?: string): string {
  return [
    'You are working on a task coordinated by Hilvan.',
    `Task id: ${task.id}. Working directory: ${cwd}.`,
    task.mode === 'read'
      ? 'This is a READ-ONLY task: do not modify files.'
      : 'Make the changes in this working directory; Hilvan will commit them to a review branch. Do not push.',
    'When done, reply with a short summary of what you did, what you verified and anything left pending.',
    '',
    brief,
    ...(handoff
      ? [
          '',
          '---',
          'Another profile worked on this task before you. Its approved continuity package follows.',
          'Treat "assumptions" and agent claims as unverified: check them in the repository.',
          '',
          handoff,
        ]
      : []),
  ].join('\n');
}

export interface RunDeps {
  host: HostContext;
  /** Effective layered policy (M3). Defaults to built-in classifications and no rules. */
  policy?: EffectivePolicy;
  /**
   * Called when a rule requires explicit approval (FR-004). Receives the exact brief that
   * will be sent. Without an approver, such runs are refused.
   */
  approve?: (evaluation: Evaluation, brief: string) => Promise<boolean>;
  git?: Git;
  criterio?: Criterio;
  runOptions?: RunOptions;
  onEvent?: (e: AgentEvent) => void;
  /** Called once the agent process started, so the CLI can wire Ctrl+C to `cancel`. */
  onStart?: (handle: RunHandle) => void;
}

export interface RunReport {
  task: Task;
  run: RunSummary;
  outcome: RunOutcome;
}

function stateFor(outcome: RunOutcome): TaskState {
  switch (outcome.state) {
    case 'completed':
      return 'in_review';
    case 'blocked':
      return 'blocked';
    case 'failed':
      return 'failed';
    case 'cancelled':
      return 'ready';
    default:
      return 'uncertain';
  }
}

/** Runs a task with a profile (FR-014, FR-016, FR-017, FR-018). */
export async function runTask(
  store: ProjectStore,
  taskId: string,
  profile: Profile,
  deps: RunDeps,
): Promise<RunReport> {
  const git = deps.git ?? defaultGit;
  await recoverStaleRun(store);
  let task = await store.loadTask(taskId);
  if (!['ready', 'failed', 'blocked', 'uncertain', 'in_review'].includes(task.state)) {
    throw new TaskError(`task is ${task.state}; it cannot run now`);
  }
  // FR-010/AC-007: never switch identity silently once a task has run with a profile.
  if (task.profileId && task.profileId !== profile.id && task.runs.length > 0) {
    throw new TaskError(
      `task ran with ${task.profileId}; continuing with ${profile.id} needs a handoff (M4)`,
    );
  }
  const policy = deps.policy ?? mergePolicies({});
  const evaluation = evaluate(policy, task, profile);
  await store.audit({
    type: 'policy.evaluated',
    taskId,
    profileId: profile.id,
    data: {
      eligible: evaluation.eligible,
      requiresApproval: evaluation.requiresApproval,
      exclusions: evaluation.exclusions.map((e) => e.source),
    },
  });
  if (!evaluation.eligible) {
    const enforced = evaluation.exclusions.some((e) => e.enforced);
    throw new TaskError(
      `profile ${profile.id} is not eligible: ${evaluation.exclusions.map((e) => e.reason).join('; ')}` +
        (enforced ? ' (enforced by the managed policy; cannot be approved)' : ''),
    );
  }
  if (evaluation.requiresApproval && !task.approvedProfiles.includes(profile.id)) {
    const brief = await store.brief(taskId);
    const ok = deps.approve ? await deps.approve(evaluation, brief) : false;
    await store.audit({
      type: ok ? 'approval.granted' : 'approval.denied',
      taskId,
      profileId: profile.id,
      data: { rules: evaluation.approvals.map((a) => a.ruleId) },
    });
    if (!ok) throw new TaskError('approval required and not granted; nothing was sent');
    task = { ...task, approvedProfiles: [...task.approvedProfiles, profile.id] };
    await store.saveTask(task);
  }

  const runId = newRunId();
  await store.acquireLock({ taskId, runId });
  const startedAt = new Date().toISOString();
  try {
    let cwd = store.root;
    if (task.mode === 'write') {
      if (!task.worktree) {
        const wt = await createWorktree(store.root, task.id, task.baseBranch ?? 'HEAD', git);
        task = { ...task, worktree: wt.path, branch: wt.branch };
        // Persist the worktree first, so a failing hook never leaves an untracked worktree.
        await store.saveTask(task);
        await runSetupHook(store, task.id, wt.path);
      }
      cwd = task.worktree as string;
    }
    task = { ...task, profileId: profile.id, state: 'running' };
    await store.saveTask(task);
    await store.audit({
      type: 'run.started',
      taskId,
      runId,
      profileId: profile.id,
      data: { destination: profile.destination.launcher, mode: task.mode },
    });

    const adapter = adapterFor(profile.app);
    // Sessions belong to an identity: only resume this profile's own session.
    const lastSession = [...task.runs]
      .reverse()
      .find((r) => r.sessionId && r.profileId === profile.id)?.sessionId;
    const cmd = adapter.runCommand(
      profile,
      { cwd, mode: task.mode, ...(lastSession ? { resumeSessionId: lastSession } : {}) },
      deps.host.env,
    );
    const inv = buildInvocation(profile.destination, cmd, deps.host);
    const latestHandoff = task.handoffs[task.handoffs.length - 1];
    const handoff =
      !lastSession && latestHandoff ? await store.handoff(taskId, latestHandoff) : undefined;
    const prompt = buildPrompt(task, await store.brief(taskId), cwd, handoff);

    const runFile = store.runFile(taskId, runId);
    await mkdir(path.dirname(runFile), { recursive: true, mode: 0o700 });
    let pending = Promise.resolve();
    const handle = startRun(
      inv,
      prompt,
      adapter.createParser(),
      (e, raw) => {
        // SR-013: run logs are stored with secrets redacted.
        const line = `${redactSecrets(raw).text}\n`;
        pending = pending.then(() => appendFile(runFile, line, { mode: 0o600 }));
        deps.onEvent?.(e);
      },
      deps.runOptions,
    );
    deps.onStart?.(handle);
    const outcome = await handle.done;
    await pending;

    const run: RunSummary = {
      id: runId,
      profileId: profile.id,
      state: outcome.state,
      ...(outcome.blockedReason ? { blockedReason: outcome.blockedReason } : {}),
      ...(outcome.sessionId ? { sessionId: outcome.sessionId } : {}),
      usage: {
        estimated: true,
        ...(outcome.costUsd !== undefined ? { costUsd: outcome.costUsd } : {}),
        ...(outcome.inputTokens !== undefined ? { inputTokens: outcome.inputTokens } : {}),
        ...(outcome.outputTokens !== undefined ? { outputTokens: outcome.outputTokens } : {}),
      },
      ...(outcome.limits ? { limits: outcome.limits } : {}),
      ...(outcome.error ? { error: outcome.error.slice(0, 2000) } : {}),
      ...(outcome.resultText
        ? { summary: redactSecrets(outcome.resultText).text.slice(0, 4000) }
        : {}),
      startedAt,
      endedAt: new Date().toISOString(),
    };

    if (task.mode === 'write' && task.worktree && task.branch) {
      await commitWorktree(task.worktree, `hilvan(${task.id}): ${task.title} [${runId}]`, git);
      const diff = await diffSummary(store.root, task.baseBranch ?? 'HEAD', task.branch, git);
      run.diff = diff;
      if (deps.criterio && diff.filesChanged) run.risk = deps.criterio.diffRisk(diff).value;
    }
    if (deps.criterio) {
      run.meetsCriteria = deps.criterio.meetsCriteria({
        state: outcome.state,
        ...(outcome.resultText ? { resultText: outcome.resultText } : {}),
        ...(outcome.permissionDenials ? { permissionDenials: outcome.permissionDenials } : {}),
        criteriaCount: task.criteriaCount,
        ...(run.diff ? { diff: run.diff } : {}),
      }).p;
    }

    task = { ...task, state: stateFor(outcome), runs: [...task.runs, run] };
    await store.saveTask(task);
    await store.audit({
      type: 'run.finished',
      taskId,
      runId,
      profileId: profile.id,
      data: {
        state: outcome.state,
        blockedReason: outcome.blockedReason,
        costUsd: outcome.costUsd,
        files: run.diff?.filesChanged,
      },
    });
    return { task, run, outcome };
  } finally {
    await store.releaseLock();
  }
}

/**
 * FR-018: optional per-project setup command run once in each new worktree (e.g. `npm ci`).
 * Declared by the user in `.hilvan/project.json`; runs without a shell, with a timeout.
 */
export async function runSetupHook(
  store: ProjectStore,
  taskId: string,
  worktree: string,
): Promise<void> {
  const project = await store.project();
  const [cmd, ...args] = project.setupHook ?? [];
  if (!cmd) return;
  const r = await runCommand(cmd, args, { cwd: worktree, timeoutMs: 15 * 60_000 });
  await store.audit({
    type: 'worktree.setup',
    taskId,
    data: { command: [cmd, ...args].join(' '), exitCode: r.code, error: r.error },
  });
  if (r.code !== 0) {
    const detail = (r.stderr || r.stdout).trim().split('\n').slice(-1)[0] ?? r.error ?? '';
    throw new TaskError(`setup hook "${[cmd, ...args].join(' ')}" failed: ${detail}`);
  }
}

/** Asks the process running a task to stop (SIGINT; the runner escalates). */
export async function cancelTask(store: ProjectStore, taskId: string): Promise<boolean> {
  const lock = await store.readLock();
  if (!lock || lock.taskId !== taskId) return false;
  if (!processAlive(lock.pid)) return false;
  process.kill(lock.pid, 'SIGINT');
  await store.audit({ type: 'run.cancel_requested', taskId, runId: lock.runId });
  return true;
}

/** FR-017: a lock whose process died leaves the task `uncertain`, never re-run automatically. */
export async function recoverStaleRun(store: ProjectStore): Promise<string | null> {
  const lock = await store.readLock();
  if (!lock || processAlive(lock.pid)) return null;
  try {
    const task = await store.loadTask(lock.taskId);
    if (task.state === 'running' && canTransition('running', 'uncertain')) {
      await store.saveTask({
        ...task,
        state: 'uncertain',
        runs: [
          ...task.runs,
          {
            id: lock.runId,
            profileId: task.profileId ?? 'unknown',
            state: 'uncertain',
            error: 'Hilvan stopped before the run finished',
            startedAt: lock.startedAt,
            endedAt: new Date().toISOString(),
          },
        ],
      });
    }
    await store.audit({ type: 'run.recovered', taskId: lock.taskId, runId: lock.runId });
  } finally {
    await store.releaseLock();
  }
  return lock.taskId;
}

/** Drops a task's worktree and branch without merging (used by `task discard`, completed in M5). */
export async function discardTask(
  store: ProjectStore,
  taskId: string,
  git: Git = defaultGit,
  /** Called with the task before it is discarded, e.g. to teach Criterio a rejection. */
  onRejected?: (task: Task) => void,
): Promise<Task> {
  let task = await store.loadTask(taskId);
  if (task.state === 'running') throw new TaskError('task is running; cancel it first');
  if (task.state === 'in_review') onRejected?.(task);
  if (task.worktree) await removeWorktree(store.root, task.id, git);
  task = { ...task, state: 'discarded', worktree: null, branch: null };
  await store.saveTask(task);
  await store.audit({ type: 'task.discarded', taskId });
  return task;
}
