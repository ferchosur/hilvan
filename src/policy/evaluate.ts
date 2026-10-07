import type { Profile, Task } from '../domain/index.js';
import type { EffectivePolicy, LayeredRule, PolicyFile, PolicyLayer } from '../domain/policy.js';
import { DEFAULT_CLASSIFICATIONS } from '../domain/policy.js';

/**
 * Merges policy layers (ADR-0009):
 * - classifications come from the user (or managed) file; projects cannot redefine them;
 * - `enforced` is honoured only in the managed layer (SR-020);
 * - project rules can only restrict: the schema has no "allow" effect (SR-019).
 */
export function mergePolicies(
  layers: Partial<Record<PolicyLayer, PolicyFile | null>>,
): EffectivePolicy {
  const warnings: string[] = [];
  const classifications = layers.managed?.classifications ??
    layers.user?.classifications ?? [...DEFAULT_CLASSIFICATIONS];
  if (layers.project?.classifications) {
    warnings.push('project policy cannot redefine classifications; ignored');
  }
  const rules: LayeredRule[] = [];
  for (const layer of ['managed', 'user', 'project'] as const) {
    for (const r of layers[layer]?.rules ?? []) {
      if (r.enforced && layer !== 'managed') {
        warnings.push(
          `rule ${r.id} (${layer}) is marked enforced; only the managed policy can enforce`,
        );
      }
      rules.push({ ...r, enforced: layer === 'managed' && r.enforced, layer });
    }
  }
  return { classifications, rules, warnings };
}

function matches(
  sel: LayeredRule['from'],
  vals: { classification?: string; profile?: Profile },
): boolean {
  if (
    sel.classification &&
    !(vals.classification && sel.classification.includes(vals.classification))
  ) {
    return false;
  }
  const p = vals.profile;
  if (sel.profiles && !(p && sel.profiles.includes(p.id))) return false;
  if (sel.tags && !(p && sel.tags.some((t) => p.tags.includes(t)))) return false;
  if (sel.apps && !(p && sel.apps.includes(p.app))) return false;
  return true;
}

export interface Exclusion {
  reason: string;
  /** Who excluded it: capability check, a policy rule, or the classification ladder. */
  source: 'classification' | 'tools' | 'routing' | `rule:${string}`;
  /** Enforced rules cannot be approved over (AC-014). */
  enforced: boolean;
}

export interface Evaluation {
  profileId: string;
  eligible: boolean;
  /** Eligible but a rule demands explicit approval for this transfer. */
  requiresApproval: boolean;
  exclusions: Exclusion[];
  approvals: Array<{ ruleId: string; reason: string; layer: PolicyLayer }>;
}

/**
 * Eligibility before optimisation (D-008, FR-011): classification accepted, required tools
 * present, no deny rule. `fromProfile` is set for hand-offs (data leaves that profile).
 */
export function evaluate(
  policy: EffectivePolicy,
  task: Pick<Task, 'classification' | 'requires'>,
  profile: Profile,
  fromProfile?: Profile,
): Evaluation {
  const exclusions: Exclusion[] = [];
  const approvals: Evaluation['approvals'] = [];
  if (!policy.classifications.includes(task.classification)) {
    exclusions.push({
      reason: `unknown classification "${task.classification}"`,
      source: 'classification',
      enforced: true,
    });
  }
  if (!profile.accepts.includes(task.classification)) {
    exclusions.push({
      reason: `does not accept "${task.classification}" data`,
      source: 'classification',
      enforced: false,
    });
  }
  const missing = task.requires.filter((r) => !profile.tools.provides.includes(r));
  if (missing.length) {
    exclusions.push({
      reason: `lacks required tools: ${missing.join(', ')}`,
      source: 'tools',
      enforced: false,
    });
  }
  for (const r of policy.rules) {
    const fromOk = matches(r.from, {
      classification: task.classification,
      ...(fromProfile ? { profile: fromProfile } : {}),
    });
    if (!fromOk || !matches(r.to, { profile })) continue;
    if (r.effect === 'deny') {
      exclusions.push({ reason: r.reason, source: `rule:${r.id}`, enforced: r.enforced });
    } else {
      approvals.push({ ruleId: r.id, reason: r.reason, layer: r.layer });
    }
  }
  const eligible = exclusions.length === 0;
  return {
    profileId: profile.id,
    eligible,
    requiresApproval: eligible && approvals.length > 0,
    exclusions,
    approvals,
  };
}
