import type { LimitWindow, RunState, Task } from '../domain/index.js';

/** Per-profile usage built only from what the CLIs reported (FR-016): estimates, never balances. */
export interface ProfileUsage {
  profileId: string;
  runs: number;
  states: Partial<Record<RunState, number>>;
  costUsd: number;
  costRuns: number;
  inputTokens: number;
  outputTokens: number;
  lastLimits?: { at: string; windows: LimitWindow[] };
  accepted: number;
  discarded: number;
  /** Cross reviews performed by this profile (their usage is included above). */
  reviews: number;
}

export function usageByProfile(tasks: readonly Task[]): ProfileUsage[] {
  const by = new Map<string, ProfileUsage>();
  const get = (id: string) => {
    let u = by.get(id);
    if (!u) {
      u = {
        profileId: id,
        runs: 0,
        states: {},
        costUsd: 0,
        costRuns: 0,
        inputTokens: 0,
        outputTokens: 0,
        accepted: 0,
        discarded: 0,
        reviews: 0,
      };
      by.set(id, u);
    }
    return u;
  };
  for (const t of tasks) {
    for (const r of t.runs) {
      const u = get(r.profileId);
      u.runs++;
      u.states[r.state] = (u.states[r.state] ?? 0) + 1;
      if (r.usage?.costUsd !== undefined) {
        u.costUsd += r.usage.costUsd;
        u.costRuns++;
      }
      u.inputTokens += r.usage?.inputTokens ?? 0;
      u.outputTokens += r.usage?.outputTokens ?? 0;
      if (r.limits?.length && (!u.lastLimits || r.startedAt > u.lastLimits.at)) {
        u.lastLimits = { at: r.startedAt, windows: r.limits };
      }
    }
    for (const r of t.reviews ?? []) {
      const u = get(r.profileId);
      u.reviews++;
      if (r.usage?.costUsd !== undefined) {
        u.costUsd += r.usage.costUsd;
        u.costRuns++;
      }
      u.inputTokens += r.usage?.inputTokens ?? 0;
      u.outputTokens += r.usage?.outputTokens ?? 0;
    }
    if (t.profileId && (t.state === 'accepted' || t.state === 'discarded')) {
      const u = get(t.profileId);
      if (t.state === 'accepted') u.accepted++;
      else u.discarded++;
    }
  }
  return [...by.values()].sort((a, b) => b.runs - a.runs);
}
