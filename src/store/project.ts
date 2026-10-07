import { constants } from 'node:fs';
import { mkdir, open, readdir, rm } from 'node:fs/promises';
import path from 'node:path';
import { z } from 'zod';
import { type Task, TaskSchema } from '../domain/index.js';
import { readTextSafe, writeFileAtomic } from '../security/fs.js';
import { type AuditEvent, appendAudit } from './audit.js';

/** Repository over the `.hilvan/` folder of a project (ADR-0007). */

export const STATE_DIR = '.hilvan';
const MAX_JSON = 4 * 1024 * 1024;

export const ProjectSchema = z.object({
  version: z.literal(1),
  name: z.string().min(1),
  baseBranch: z.string().min(1),
  /** Optional command run in each new worktree (e.g. `npm ci`), split on spaces, no shell. */
  setupHook: z.array(z.string()).optional(),
  createdAt: z.iso.datetime(),
});
export type Project = z.infer<typeof ProjectSchema>;

export class ProjectError extends Error {
  override name = 'ProjectError';
}

const TASK_ID = /^tsk_[0-9A-Z]{26}$/;
const RUN_ID = /^run_[0-9A-Z]{26}$/;

export class ProjectStore {
  readonly dir: string;
  constructor(readonly root: string) {
    this.dir = path.join(root, STATE_DIR);
  }

  /** Walks up from `cwd` to the nearest folder containing `.hilvan/project.json`. */
  static async find(cwd: string): Promise<ProjectStore> {
    let dir = path.resolve(cwd);
    for (;;) {
      const text = await readTextSafe(path.join(dir, STATE_DIR, 'project.json'), MAX_JSON).catch(
        () => null,
      );
      if (text !== null) return new ProjectStore(dir);
      const parent = path.dirname(dir);
      if (parent === dir) throw new ProjectError('not a Hilvan project (run `hilvan init`)');
      dir = parent;
    }
  }

  async init(project: Omit<Project, 'version' | 'createdAt'>): Promise<Project> {
    const full: Project = { version: 1, createdAt: new Date().toISOString(), ...project };
    await mkdir(this.dir, { recursive: true, mode: 0o700 });
    // SR-016: the whole state folder stays out of git without touching the user's .gitignore.
    await writeFileAtomic(path.join(this.dir, '.gitignore'), '*\n', 0o644);
    await writeFileAtomic(
      path.join(this.dir, 'project.json'),
      `${JSON.stringify(full, null, 2)}\n`,
    );
    return full;
  }

  async project(): Promise<Project> {
    const text = await readTextSafe(path.join(this.dir, 'project.json'), MAX_JSON);
    if (text === null) throw new ProjectError('missing .hilvan/project.json');
    return ProjectSchema.parse(JSON.parse(text));
  }

  taskDir(id: string): string {
    if (!TASK_ID.test(id)) throw new ProjectError(`invalid task id "${id}"`);
    return path.join(this.dir, 'tasks', id);
  }

  runFile(taskId: string, runId: string): string {
    if (!RUN_ID.test(runId)) throw new ProjectError(`invalid run id "${runId}"`);
    return path.join(this.taskDir(taskId), 'runs', `${runId}.jsonl`);
  }

  async saveTask(task: Task, brief?: string): Promise<void> {
    const valid = TaskSchema.parse({ ...task, updatedAt: new Date().toISOString() });
    const dir = this.taskDir(valid.id);
    await mkdir(path.join(dir, 'runs'), { recursive: true, mode: 0o700 });
    await writeFileAtomic(path.join(dir, 'task.json'), `${JSON.stringify(valid, null, 2)}\n`);
    if (brief !== undefined) await writeFileAtomic(path.join(dir, 'brief.md'), brief);
  }

  async loadTask(id: string): Promise<Task> {
    const text = await readTextSafe(path.join(this.taskDir(id), 'task.json'), MAX_JSON);
    if (text === null) throw new ProjectError(`no task "${id}"`);
    return TaskSchema.parse(JSON.parse(text));
  }

  async brief(id: string): Promise<string> {
    return (await readTextSafe(path.join(this.taskDir(id), 'brief.md'), MAX_JSON)) ?? '';
  }

  /** Resolves a full id or a unique suffix/prefix (e.g. the last 6 characters shown in lists). */
  async resolveTaskId(ref: string): Promise<string> {
    if (TASK_ID.test(ref)) return ref;
    const ids = await this.taskIds();
    const hits = ids.filter((id) => id.endsWith(ref.toUpperCase()) || id.startsWith(ref));
    if (hits.length === 1 && hits[0]) return hits[0];
    throw new ProjectError(hits.length ? `ambiguous task "${ref}"` : `no task "${ref}"`);
  }

  async taskIds(): Promise<string[]> {
    try {
      return (await readdir(path.join(this.dir, 'tasks'))).filter((n) => TASK_ID.test(n)).sort();
    } catch {
      return [];
    }
  }

  async listTasks(): Promise<Task[]> {
    return Promise.all((await this.taskIds()).map((id) => this.loadTask(id)));
  }

  audit(event: Omit<AuditEvent, 'ts'>): Promise<unknown> {
    return appendAudit(path.join(this.dir, 'events.jsonl'), {
      ts: new Date().toISOString(),
      ...event,
    });
  }

  // ── Single-run lock (one active run per project in the MVP) ───────────────

  get lockFile(): string {
    return path.join(this.dir, 'lock');
  }

  async acquireLock(info: { taskId: string; runId: string }): Promise<void> {
    const body = JSON.stringify({ pid: process.pid, ...info, startedAt: new Date().toISOString() });
    try {
      const fh = await open(
        this.lockFile,
        constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL,
        0o600,
      );
      await fh.writeFile(body);
      await fh.close();
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code === 'EEXIST') {
        const held = await this.readLock();
        throw new ProjectError(
          `another run is active (${held?.runId ?? 'unknown'} for ${held?.taskId ?? '?'}, pid ${held?.pid ?? '?'})`,
        );
      }
      throw err;
    }
  }

  async readLock(): Promise<{
    pid: number;
    taskId: string;
    runId: string;
    startedAt: string;
  } | null> {
    const text = await readTextSafe(this.lockFile, 4096).catch(() => null);
    if (!text) return null;
    try {
      return JSON.parse(text);
    } catch {
      return null;
    }
  }

  async releaseLock(): Promise<void> {
    await rm(this.lockFile, { force: true });
  }
}

/** True when a process with this pid exists (signal 0 probes without sending anything). */
export function processAlive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return (err as NodeJS.ErrnoException).code === 'EPERM';
  }
}
