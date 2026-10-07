import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import type { DiffSummary } from '../domain/index.js';
import { type RunResult, runCommand } from '../process/run.js';

/**
 * Git operations for task isolation (ADR-0008, SR-003, SR-023).
 * Branch and worktree names come only from validated task ids.
 */

export class GitError extends Error {
  override name = 'GitError';
}

export type Git = (args: readonly string[], cwd: string) => Promise<RunResult>;

export const defaultGit: Git = (args, cwd) =>
  runCommand('git', args, {
    cwd,
    timeoutMs: 60_000,
    // Never prompt for credentials or open editors from a background run.
    env: { ...process.env, GIT_TERMINAL_PROMPT: '0', GIT_EDITOR: 'true' },
  });

async function ok(git: Git, args: readonly string[], cwd: string): Promise<string> {
  const r = await git(args, cwd);
  if (r.code !== 0) {
    throw new GitError(
      `git ${args[0]} failed: ${(r.stderr || r.stdout).trim().split('\n')[0] ?? r.error}`,
    );
  }
  return r.stdout.trim();
}

const TASK_ID = /^tsk_[0-9A-Z]{26}$/;

export function taskBranch(taskId: string): string {
  if (!TASK_ID.test(taskId)) throw new GitError(`invalid task id ${taskId}`);
  return `hilvan/${taskId}`;
}

/** Worktrees live next to the repository, never inside it: `<parent>/.hilvan-worktrees/<repo>/<task>`. */
export function worktreePath(root: string, taskId: string): string {
  taskBranch(taskId);
  return path.join(path.dirname(root), '.hilvan-worktrees', path.basename(root), taskId);
}

export async function repoRoot(cwd: string, git: Git = defaultGit): Promise<string> {
  return ok(git, ['rev-parse', '--show-toplevel'], cwd);
}

export async function currentBranch(root: string, git: Git = defaultGit): Promise<string> {
  return ok(git, ['rev-parse', '--abbrev-ref', 'HEAD'], root);
}

export async function createWorktree(
  root: string,
  taskId: string,
  base: string,
  git: Git = defaultGit,
): Promise<{ path: string; branch: string }> {
  const branch = taskBranch(taskId);
  const wt = worktreePath(root, taskId);
  await mkdir(path.dirname(wt), { recursive: true, mode: 0o700 });
  await ok(git, ['worktree', 'add', '-b', branch, '--', wt, base], root);
  return { path: wt, branch };
}

export async function removeWorktree(
  root: string,
  taskId: string,
  git: Git = defaultGit,
): Promise<void> {
  await ok(git, ['worktree', 'remove', '--force', '--', worktreePath(root, taskId)], root);
  await ok(git, ['branch', '-D', '--', taskBranch(taskId)], root);
}

/**
 * Commits everything the agent changed in the worktree on the task branch.
 * Repository hooks are skipped (`--no-verify`): they are untrusted repository code (T10).
 * Returns false when there was nothing to commit.
 */
export async function commitWorktree(
  wt: string,
  message: string,
  git: Git = defaultGit,
): Promise<boolean> {
  await ok(git, ['add', '-A'], wt);
  const staged = await git(['diff', '--cached', '--quiet'], wt);
  if (staged.code === 0) return false;
  const name = (await git(['config', 'user.name'], wt)).stdout.trim();
  const identity = name ? [] : ['-c', 'user.name=Hilvan', '-c', 'user.email=hilvan@localhost'];
  await ok(git, [...identity, 'commit', '--no-verify', '-q', '-m', message], wt);
  return true;
}

/** Numstat of `base...branch` as Criterio diff signals. */
export async function diffSummary(
  root: string,
  base: string,
  branch: string,
  git: Git = defaultGit,
): Promise<DiffSummary> {
  const out = await ok(git, ['diff', '--numstat', '-z', `${base}...${branch}`], root);
  let added = 0;
  let deleted = 0;
  const paths: string[] = [];
  // -z format: "<added>\t<deleted>\t<path>\0" (renames use extra NUL-separated fields)
  const parts = out.split('\0').filter(Boolean);
  for (let i = 0; i < parts.length; i++) {
    const m = /^(\d+|-)\t(\d+|-)\t(.*)$/.exec(parts[i] ?? '');
    if (!m) continue;
    added += m[1] === '-' ? 0 : Number(m[1]);
    deleted += m[2] === '-' ? 0 : Number(m[2]);
    if (m[3]) paths.push(m[3]);
    else {
      // rename: next two fields are old and new paths
      const to = parts[i + 2];
      if (to) paths.push(to);
      i += 2;
    }
  }
  return {
    filesChanged: paths.length,
    linesAdded: added,
    linesDeleted: deleted,
    paths: paths.slice(0, 2000),
  };
}
