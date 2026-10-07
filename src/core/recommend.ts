import type { Criterio, Reason, TaskType } from '../criterio/index.js';
import type { Profile, Task } from '../domain/index.js';
import type { EffectivePolicy } from '../domain/policy.js';
import { type Evaluation, evaluate } from '../policy/evaluate.js';

export interface Recommendation {
  taskType: TaskType;
  ranked: Array<{ profile: Profile; evaluation: Evaluation; score: number; why: Reason[] }>;
  excluded: Array<{ profile: Profile; evaluation: Evaluation }>;
}

/**
 * Remaining share of the tightest quota window reported by the profile's latest run
 * (FR-012: only data the provider exposes). A recent rate-limit block counts as zero.
 */
export function quotaHeadroom(
  profileId: string,
  tasks: readonly Task[],
  now = Date.now(),
): number | undefined {
  const runs = tasks
    .flatMap((t) => t.runs)
    .filter((r) => r.profileId === profileId)
    .sort((a, b) => (a.startedAt < b.startedAt ? 1 : -1));
  const latest = runs[0];
  if (!latest) return undefined;
  if (latest.blockedReason === 'rate_limit' && now - Date.parse(latest.startedAt) < 5 * 3600_000)
    return 0;
  const withLimits = runs.find((r) => r.limits?.length);
  if (!withLimits?.limits) return undefined;
  return 1 - Math.max(...withLimits.limits.map((w) => w.utilization));
}

/** Eligibility first, then Criterio ranks only the eligible profiles (D-008, D-009). */
export function recommend(
  policy: EffectivePolicy,
  task: Task,
  profiles: readonly Profile[],
  criterio: Criterio,
  history: readonly Task[],
  seed?: number,
): Recommendation {
  const taskType = task.taskType ?? criterio.taskType({ title: task.title }).value;
  const evals = profiles.map((p) => ({ profile: p, evaluation: evaluate(policy, task, p) }));
  // FR-021: reserved profiles only take tasks that need one of their tools, unless nothing else
  // is eligible (then they stay, flagged), so "everything else" goes to default profiles.
  const needsOwnTool = (p: Profile) => task.requires.some((r) => p.tools.provides.includes(r));
  const eligibleAll = evals.filter((e) => e.evaluation.eligible);
  const defaults = eligibleAll.filter(
    (e) => e.profile.routing !== 'reserved' || needsOwnTool(e.profile),
  );
  const reservedOut = eligibleAll.filter((e) => !defaults.includes(e));
  const useReserved = defaults.length === 0;
  const eligible = useReserved ? eligibleAll : defaults;
  const routedOut = useReserved
    ? []
    : reservedOut.map((e) => ({
        profile: e.profile,
        evaluation: {
          ...e.evaluation,
          eligible: false,
          exclusions: [
            {
              reason: `reserved for tasks that need ${e.profile.tools.provides.join(', ') || 'its tools'}`,
              source: 'routing' as const,
              enforced: false,
            },
          ],
        },
      }));
  const ranking = criterio.rankProfiles(
    eligible.map(({ profile: p }) => {
      const headroom = quotaHeadroom(p.id, history);
      return {
        id: p.id,
        app: p.app,
        preference: p.preference,
        ...(headroom !== undefined ? { quotaHeadroom: headroom } : {}),
      };
    }),
    taskType,
    seed,
  );
  const byId = new Map(eligible.map((e) => [e.profile.id, e]));
  return {
    taskType,
    ranked: ranking.flatMap((r) => {
      const e = byId.get(r.id);
      if (!e) return [];
      const why = [...r.why];
      if (e.profile.routing === 'reserved') {
        why.unshift({
          signal: needsOwnTool(e.profile)
            ? `reservado: la tarea requiere ${task.requires.filter((x) => e.profile.tools.provides.includes(x)).join(', ')}`
            : 'reservado, pero es el único perfil elegible',
          weight: 1,
        });
      }
      return [{ ...e, score: r.score, why }];
    }),
    excluded: [...evals.filter((e) => !e.evaluation.eligible), ...routedOut],
  };
}
