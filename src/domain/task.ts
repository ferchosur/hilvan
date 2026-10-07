import { z } from 'zod';
import { TASK_TYPES } from '../criterio/types.js';
import { IdSchema } from './profile.js';

/** Task lifecycle (docs/design/modelo-de-datos.md §5). */
export const TaskStateSchema = z.enum([
  'draft',
  'ready',
  'awaiting_approval',
  'running',
  'in_review',
  'failed',
  'blocked',
  'uncertain',
  'accepted',
  'discarded',
]);
export type TaskState = z.infer<typeof TaskStateSchema>;

const TRANSITIONS: Readonly<Record<TaskState, readonly TaskState[]>> = {
  draft: ['ready'],
  ready: ['awaiting_approval', 'running'],
  awaiting_approval: ['ready', 'draft'],
  running: ['in_review', 'failed', 'blocked', 'uncertain', 'ready'],
  in_review: ['ready', 'accepted', 'discarded'],
  failed: ['ready'],
  blocked: ['ready'],
  uncertain: ['ready'],
  accepted: [],
  discarded: [],
};

export function canTransition(from: TaskState, to: TaskState): boolean {
  return TRANSITIONS[from].includes(to);
}

export function isTerminal(state: TaskState): boolean {
  return TRANSITIONS[state].length === 0;
}

export const RunStateSchema = z.enum([
  'running',
  'completed',
  'failed',
  'blocked',
  'cancelled',
  'uncertain',
]);
export type RunState = z.infer<typeof RunStateSchema>;

export const BlockedReasonSchema = z.enum(['rate_limit', 'auth', 'billing', 'permission']);
export type BlockedReason = z.infer<typeof BlockedReasonSchema>;

/** Usage reported by the agent CLI; always a client-side estimate (FR-016). */
export const UsageSchema = z.object({
  costUsd: z.number().nonnegative().optional(),
  inputTokens: z.number().int().nonnegative().optional(),
  outputTokens: z.number().int().nonnegative().optional(),
  estimated: z.literal(true),
});
export type Usage = z.infer<typeof UsageSchema>;

export const LimitWindowSchema = z.object({
  name: z.string(),
  utilization: z.number().min(0).max(1),
  resetsAt: z.number().optional(),
});
export type LimitWindow = z.infer<typeof LimitWindowSchema>;

export const DiffSummarySchema = z.object({
  filesChanged: z.number().int().nonnegative(),
  linesAdded: z.number().int().nonnegative(),
  linesDeleted: z.number().int().nonnegative(),
  paths: z.array(z.string()).max(2000),
});
export type DiffSummary = z.infer<typeof DiffSummarySchema>;

export const RunSummarySchema = z.object({
  id: z.string().regex(/^run_[0-9A-Z]{26}$/),
  profileId: IdSchema,
  state: RunStateSchema,
  blockedReason: BlockedReasonSchema.optional(),
  sessionId: z.string().optional(),
  usage: UsageSchema.optional(),
  limits: z.array(LimitWindowSchema).optional(),
  diff: DiffSummarySchema.optional(),
  /** Criterio diff risk, 2..10. */
  risk: z.number().min(2).max(10).optional(),
  /** Criterio estimate that the run met the brief. */
  meetsCriteria: z.number().min(0).max(1).optional(),
  error: z.string().max(2000).optional(),
  /** Agent's final summary, secrets redacted, truncated. An unverified claim. */
  summary: z.string().max(4000).optional(),
  startedAt: z.iso.datetime(),
  endedAt: z.iso.datetime().optional(),
});
export type RunSummary = z.infer<typeof RunSummarySchema>;

export const TaskSchema = z.object({
  id: z.string().regex(/^tsk_[0-9A-Z]{26}$/),
  title: z.string().min(1),
  classification: z.string().min(1),
  /** `write` tasks run in their own git worktree (ADR-0008). */
  mode: z.enum(['write', 'read']),
  requires: z.array(IdSchema).default([]),
  taskType: z.enum(TASK_TYPES).optional(),
  complexity: z.number().min(2).max(10).optional(),
  criteriaCount: z.number().int().nonnegative().default(0),
  profileId: IdSchema.nullable().default(null),
  baseBranch: z.string().nullable().default(null),
  state: TaskStateSchema,
  branch: z.string().nullable().default(null),
  worktree: z.string().nullable().default(null),
  runs: z.array(RunSummarySchema).default([]),
  handoffs: z.array(z.string().regex(/^ho_[0-9A-Z]{26}$/)).default([]),
  /** Cross reviews attached to the task (FR-015). */
  reviews: z
    .array(
      z.object({
        id: z.string().regex(/^run_[0-9A-Z]{26}$/),
        profileId: IdSchema,
        verdict: z.enum(['approve', 'changes_requested', 'unknown']),
        at: z.iso.datetime(),
      }),
    )
    .default([]),
  /** Branch head whose diff the user reviewed; accepting requires it to be current (SR-024). */
  reviewedHead: z.string().nullable().default(null),
  /** Profiles the user already approved to receive this task's brief (run approval or hand-off). */
  approvedProfiles: z.array(IdSchema).default([]),
  createdAt: z.iso.datetime(),
  updatedAt: z.iso.datetime(),
});
export type Task = z.infer<typeof TaskSchema>;
