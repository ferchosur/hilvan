import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import { type AgentEvent, adapterFor } from '../adapters/index.js';
import { startRun } from '../adapters/runner.js';
import type { Criterio } from '../criterio/index.js';
import { newRunId } from '../domain/ids.js';
import type { Profile, Task } from '../domain/index.js';
import { defaultGit, type Git, GitError, removeWorktree } from '../gitops/index.js';
import { buildInvocation } from '../launchers/index.js';
import { evaluate, mergePolicies } from '../policy/evaluate.js';
import { writeFileAtomic } from '../security/fs.js';
import { redactSecrets } from '../security/secrets.js';
import type { ProjectStore } from '../store/project.js';
import { type RunDeps, TaskError } from './tasks.js';

/** M5 — review, cross review and acceptance (FR-015, AC-010, AC-018, AC-021, SR-024). */

const MAX_DIFF = 2 * 1024 * 1024;

async function headOf(store: ProjectStore, branch: string, git: Git): Promise<string> {
  const r = await git(['rev-parse', '--verify', `${branch}^{commit}`], store.root);
  if (r.code !== 0) throw new GitError(`branch ${branch} not found`);
  return r.stdout.trim();
}

/** The task's diff against its base, bounded in size. Marks the current head as reviewed. */
export async function reviewDiff(
  store: ProjectStore,
  taskId: string,
  git: Git = defaultGit,
): Promise<{ task: Task; diff: string; truncated: boolean }> {
  const task = await store.loadTask(taskId);
  if (!task.branch || !task.baseBranch) return { task, diff: '', truncated: false };
  const r = await git(
    ['diff', '--no-color', '--no-ext-diff', `${task.baseBranch}...${task.branch}`],
    store.root,
  );
  if (r.code !== 0) throw new GitError(`git diff failed: ${r.stderr.trim()}`);
  const head = await headOf(store, task.branch, git);
  const updated = { ...task, reviewedHead: head };
  await store.saveTask(updated);
  await store.audit({ type: 'review.viewed', taskId, data: { head } });
  const truncated = r.stdout.length > MAX_DIFF;
  return { task: updated, diff: truncated ? r.stdout.slice(0, MAX_DIFF) : r.stdout, truncated };
}

export function reviewPrompt(task: Task, brief: string): string {
  const range = task.branch && task.baseBranch ? `${task.baseBranch}...${task.branch}` : null;
  return [
    'You are an independent reviewer. This is a READ-ONLY task: do not modify any file.',
    range
      ? `Inspect the changes with \`git diff ${range}\` and \`git log ${task.baseBranch}..${task.branch}\`.`
      : 'There is no code change; evaluate the previous result described below against the brief.',
    'Check every acceptance criterion of the brief, look for bugs, security issues and unverified claims.',
    'Answer with a short list of findings (severity: high/medium/low), then a final line exactly:',
    'VERDICT: approve   or   VERDICT: changes_requested',
    '',
    brief,
    ...(task.runs.at(-1)?.summary
      ? ['', 'Previous agent summary (unverified):', task.runs.at(-1)?.summary ?? '']
      : []),
  ].join('\n');
}

export function parseVerdict(text: string): 'approve' | 'changes_requested' | 'unknown' {
  const m = /VERDICT:\s*(approve|changes_requested)/i.exec(text);
  return m ? (m[1]?.toLowerCase() as 'approve' | 'changes_requested') : 'unknown';
}

/** Runs a read-only review with another profile; the review is attached to the task (AC-018). */
export async function crossReview(
  store: ProjectStore,
  taskId: string,
  reviewer: Profile,
  deps: RunDeps,
): Promise<{
  task: Task;
  verdict: 'approve' | 'changes_requested' | 'unknown';
  text: string;
  file: string;
}> {
  let task = await store.loadTask(taskId);
  if (!task.runs.length) throw new TaskError('nothing to review yet: run the task first');
  if (task.state === 'running') throw new TaskError('the task is running');
  if (reviewer.id === task.profileId)
    throw new TaskError('a cross review needs a different profile');
  const evaluation = evaluate(deps.policy ?? mergePolicies({}), task, reviewer);
  if (!evaluation.eligible) {
    throw new TaskError(
      `profile ${reviewer.id} is not eligible to review: ${evaluation.exclusions.map((e) => e.reason).join('; ')}`,
    );
  }
  const brief = await store.brief(taskId);
  if (evaluation.requiresApproval && !task.approvedProfiles.includes(reviewer.id)) {
    const ok = deps.approve ? await deps.approve(evaluation, brief) : false;
    await store.audit({
      type: ok ? 'approval.granted' : 'approval.denied',
      taskId,
      profileId: reviewer.id,
    });
    if (!ok) throw new TaskError('approval required and not granted; nothing was sent');
    task = { ...task, approvedProfiles: [...task.approvedProfiles, reviewer.id] };
    await store.saveTask(task);
  }

  const runId = newRunId();
  await store.acquireLock({ taskId, runId });
  try {
    const cwd = task.worktree ?? store.root;
    const adapter = adapterFor(reviewer.app);
    const inv = buildInvocation(
      reviewer.destination,
      adapter.runCommand(reviewer, { cwd, mode: 'read' }, deps.host.env),
      deps.host,
    );
    await store.audit({ type: 'review.started', taskId, runId, profileId: reviewer.id });
    const messages: string[] = [];
    const handle = startRun(
      inv,
      reviewPrompt(task, brief),
      adapter.createParser(),
      (e: AgentEvent) => {
        if (e.type === 'message') messages.push(e.text);
        deps.onEvent?.(e);
      },
      deps.runOptions,
    );
    deps.onStart?.(handle);
    const outcome = await handle.done;
    const text = redactSecrets(outcome.resultText || messages.join('\n\n')).text;
    const verdict = outcome.state === 'completed' ? parseVerdict(text) : 'unknown';
    const file = store.reviewFile(taskId, runId);
    await mkdir(path.dirname(file), { recursive: true, mode: 0o700 });
    await writeFileAtomic(
      file,
      `# Revisión cruzada por ${reviewer.id}\n\n- Estado: ${outcome.state}\n- Veredicto: ${verdict}\n\n${text}\n`,
    );
    task = {
      ...(await store.loadTask(taskId)),
    };
    task = {
      ...task,
      reviews: [
        ...task.reviews,
        {
          id: runId,
          profileId: reviewer.id,
          verdict,
          usage: {
            estimated: true,
            ...(outcome.costUsd !== undefined ? { costUsd: outcome.costUsd } : {}),
            ...(outcome.inputTokens !== undefined ? { inputTokens: outcome.inputTokens } : {}),
            ...(outcome.outputTokens !== undefined ? { outputTokens: outcome.outputTokens } : {}),
          },
          at: new Date().toISOString(),
        },
      ],
    };
    await store.saveTask(task);
    await store.audit({
      type: 'review.attached',
      taskId,
      runId,
      profileId: reviewer.id,
      data: { verdict, state: outcome.state },
    });
    return { task, verdict, text, file };
  } finally {
    await store.releaseLock();
  }
}

/**
 * Accepts a task (AC-010, AC-021): write tasks are merged into the base branch only after the
 * current diff was shown, with a clean checkout on the base branch; the worktree is removed.
 */
export async function acceptTask(
  store: ProjectStore,
  taskId: string,
  criterio?: Criterio,
  git: Git = defaultGit,
): Promise<Task> {
  let task = await store.loadTask(taskId);
  if (task.state !== 'in_review')
    throw new TaskError(`only tasks in review can be accepted (task is ${task.state})`);
  if (task.mode === 'write' && task.branch && task.baseBranch) {
    const head = await headOf(store, task.branch, git);
    if (task.reviewedHead !== head) {
      throw new TaskError('review the current diff first: hilvan review <task> (SR-024)');
    }
    const current = (await git(['rev-parse', '--abbrev-ref', 'HEAD'], store.root)).stdout.trim();
    if (current !== task.baseBranch) {
      throw new TaskError(
        `check out ${task.baseBranch} in ${store.root} before accepting (now on ${current})`,
      );
    }
    const dirty = (
      await git(['status', '--porcelain', '--untracked-files=no'], store.root)
    ).stdout.trim();
    if (dirty)
      throw new TaskError('the main checkout has uncommitted changes; commit or stash them first');
    const merge = await git(
      [
        'merge',
        '--no-ff',
        '--no-verify',
        '-m',
        `hilvan: ${task.title} (${task.id})`,
        '--',
        task.branch,
      ],
      store.root,
    );
    if (merge.code !== 0) {
      await git(['merge', '--abort'], store.root);
      throw new TaskError(`merge failed (conflicts?): ${merge.stderr.trim().split('\n')[0] ?? ''}`);
    }
    await removeWorktree(store.root, task.id, git);
  }
  task = { ...task, state: 'accepted', worktree: null, branch: null };
  await store.saveTask(task);
  learnOutcome(task, true, criterio);
  await store.audit({
    type: 'task.accepted',
    taskId,
    ...(task.profileId ? { profileId: task.profileId } : {}),
  });
  return task;
}

/** Feeds the human decision back into Criterio (profile ranking and acceptance model). */
export function learnOutcome(task: Task, accepted: boolean, criterio?: Criterio): void {
  if (!criterio || !task.profileId) return;
  const last = task.runs.at(-1);
  criterio.recordOutcome(task.profileId, task.taskType ?? 'implement', accepted);
  if (last) {
    criterio.learnAcceptance(
      {
        state: last.state === 'running' ? 'uncertain' : last.state,
        ...(last.summary ? { resultText: last.summary } : {}),
        criteriaCount: task.criteriaCount,
        ...(last.diff ? { diff: last.diff } : {}),
      },
      accepted,
    );
  }
}
