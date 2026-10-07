import {
  armMean,
  armObservations,
  type BetaArm,
  mulberry32,
  PRIOR,
  sampleBeta,
  updateArm,
} from './beta.js';
import { brier, type Outcome, reliability } from './calibration.js';
import { diffFeatures, type Feature, taskFeatures, tokenize } from './features.js';
import { OnlineSoftmax, type SoftmaxState } from './logistic.js';
import {
  acceptanceRules,
  complexityRules,
  diffRiskRules,
  sensitivityRules,
  taskTypeRules,
} from './rules.js';
import { DEFAULT_THRESHOLDS, disposition, type Thresholds } from './thresholds.js';
import {
  type Action,
  type Decision,
  type DiffSignals,
  type Disposition,
  type Reason,
  type RunSignals,
  TASK_TYPES,
  type TaskSignals,
  type TaskType,
} from './types.js';

/** Examples needed before the learned model weighs as much as the rules. */
const BLEND_K = 10;
const HISTORY_MAX = 500;

export type Question = 'taskType' | 'sensitivity' | 'acceptance';

export interface CriterioState {
  version: 1;
  taskType: SoftmaxState;
  sensitivity: SoftmaxState;
  acceptance: SoftmaxState;
  /** Learned complexity offset per task type (user corrections minus rule score). */
  complexityOffset: Partial<Record<TaskType, { sum: number; n: number }>>;
  /** Success counts per `profileId|taskType`. */
  arms: Record<string, BetaArm>;
  history: Array<{ q: Question; p: number; y: 0 | 1 }>;
}

export interface Candidate {
  id: string;
  /** Application, used so quota headroom never separates profiles of the same app (FR-012). */
  app: string;
  preference?: number;
  /** Remaining share of the tightest known quota window, 0..1; undefined when not reported. */
  quotaHeadroom?: number;
}

export interface Ranked {
  id: string;
  score: number;
  why: Reason[];
}

function blend(rules: number[], model: number[], n: number): { probs: number[]; alpha: number } {
  const alpha = n / (n + BLEND_K);
  return { probs: rules.map((r, i) => (1 - alpha) * r + alpha * (model[i] ?? 0)), alpha };
}

function argmax(xs: readonly number[]): number {
  let best = 0;
  for (let i = 1; i < xs.length; i++) if ((xs[i] ?? 0) > (xs[best] ?? 0)) best = i;
  return best;
}

function mergeWhy(a: Reason[], b: Reason[], top = 5): Reason[] {
  return [...a, ...b].sort((x, y) => Math.abs(y.weight) - Math.abs(x.weight)).slice(0, top);
}

function source(alpha: number): Decision<unknown>['source'] {
  return alpha === 0 ? 'rules' : alpha > 0.95 ? 'model' : 'blend';
}

function runFeatures(r: RunSignals): Feature[] {
  const feats: Feature[] = [
    { name: 'bias', value: 1 },
    { name: `state:${r.state}`, value: 1 },
  ];
  for (const t of new Set(tokenize(r.resultText ?? '').slice(0, 200))) {
    feats.push({ name: `r:${t}`, value: 1 });
  }
  if (r.permissionDenials) feats.push({ name: 'denials', value: Math.min(r.permissionDenials, 5) });
  if (r.diff) feats.push(...diffFeatures(r.diff));
  return feats;
}

/** Hilvan's local decision engine (ADR-0011). Deterministic given its state and seed. */
export class Criterio {
  private readonly taskTypeModel: OnlineSoftmax;
  private readonly sensitivityModel: OnlineSoftmax;
  private readonly acceptanceModel: OnlineSoftmax;
  private readonly complexityOffset: CriterioState['complexityOffset'];
  private readonly arms: Map<string, BetaArm>;
  private readonly history: CriterioState['history'];
  readonly thresholds: Thresholds;

  constructor(state?: CriterioState, thresholds: Thresholds = DEFAULT_THRESHOLDS) {
    this.taskTypeModel = state
      ? OnlineSoftmax.fromJSON(state.taskType)
      : new OnlineSoftmax(TASK_TYPES.length);
    this.sensitivityModel = state
      ? OnlineSoftmax.fromJSON(state.sensitivity)
      : new OnlineSoftmax(2);
    this.acceptanceModel = state ? OnlineSoftmax.fromJSON(state.acceptance) : new OnlineSoftmax(2);
    this.complexityOffset = { ...(state?.complexityOffset ?? {}) };
    this.arms = new Map(Object.entries(state?.arms ?? {}));
    this.history = [...(state?.history ?? [])];
    this.thresholds = thresholds;
  }

  // ── Questions ────────────────────────────────────────────────────────────

  /** choice: what kind of task is this? */
  taskType(s: TaskSignals): Decision<TaskType> {
    const rules = taskTypeRules(s);
    const feats = taskFeatures(s);
    const n = this.taskTypeModel.updates;
    const { probs, alpha } = blend(rules.probs, n ? this.taskTypeModel.predict(feats) : [], n);
    const i = argmax(probs);
    const why = mergeWhy(rules.why.slice(0, 4), n ? this.taskTypeModel.explain(feats, i) : []);
    return { value: TASK_TYPES[i] ?? 'implement', p: probs[i] ?? 0, why, source: source(alpha) };
  }

  /** score: how complex is the task (2..10)? */
  complexity(s: TaskSignals, type?: TaskType): Decision<number> {
    const r = complexityRules(s);
    const off = type ? this.complexityOffset[type] : undefined;
    const shift = off?.n ? off.sum / off.n : 0;
    const words = tokenize(`${s.title ?? ''} ${s.text ?? ''}`).length;
    const why = [...r.why];
    if (Math.abs(shift) >= 0.2) {
      why.push({ signal: `ajuste aprendido para tareas ${type}`, weight: shift });
    }
    const value = Math.round(Math.min(10, Math.max(2, r.score + shift)) * 10) / 10;
    return {
      value,
      p: Math.min(0.9, 0.45 + 0.45 * Math.min(1, words / 40)),
      why,
      source: off?.n ? 'blend' : 'rules',
    };
  }

  /** score: how risky is this diff (2..10)? Facts-based, so confidence is high. */
  diffRisk(d: DiffSignals): Decision<number> {
    const r = diffRiskRules(d);
    return { value: r.score, p: 0.8, why: r.why, source: 'rules' };
  }

  /** yesno: does the brief look corporate/personal-sensitive? Used only to harden classification. */
  sensitive(s: TaskSignals): Decision<boolean> {
    const rules = sensitivityRules(s);
    const feats = taskFeatures(s);
    const n = this.sensitivityModel.updates;
    const model = n ? this.sensitivityModel.predict(feats) : [];
    const { probs, alpha } = blend([1 - rules.p, rules.p], model, n);
    const p1 = probs[1] ?? 0;
    const value = p1 >= 0.5;
    const why = mergeWhy(rules.why, n ? this.sensitivityModel.explain(feats, value ? 1 : 0) : []);
    return { value, p: value ? p1 : 1 - p1, why, source: source(alpha) };
  }

  /**
   * Suggests a stricter classification when the brief looks sensitive.
   * Never proposes a less strict one. `order` lists classifications from least to most strict.
   */
  suggestClassification(
    s: TaskSignals,
    current: string,
    order: readonly string[] = ['public', 'personal', 'corporate', 'confidential'],
  ): Decision<string | null> {
    const d = this.sensitive(s);
    const target = order.includes('corporate') ? 'corporate' : (order[order.length - 1] ?? current);
    const harden = d.value && order.indexOf(current) < order.indexOf(target);
    return { value: harden ? target : null, p: d.p, why: d.why, source: d.source };
  }

  /** yesno: did the run satisfy the brief? */
  meetsCriteria(r: RunSignals): Decision<boolean> {
    const rules = acceptanceRules(r);
    const feats = runFeatures(r);
    const n = this.acceptanceModel.updates;
    const model = n ? this.acceptanceModel.predict(feats) : [];
    const { probs, alpha } = blend([1 - rules.p, rules.p], model, n);
    const p1 = probs[1] ?? 0;
    const value = p1 >= 0.5;
    const why = mergeWhy(rules.why, n ? this.acceptanceModel.explain(feats, value ? 1 : 0) : []);
    return { value, p: value ? p1 : 1 - p1, why, source: source(alpha) };
  }

  /** What Hilvan may do with a decision on its own (auto / confirm / escalate). */
  dispose(d: Decision<unknown>, action: Action, risk = 0): Disposition {
    return disposition(d.p, action, risk, this.thresholds);
  }

  /**
   * Orders eligible profiles for a task type: Thompson sample of past success,
   * quota headroom (equalised within each app) and user preference.
   */
  rankProfiles(candidates: readonly Candidate[], type: TaskType, seed = Date.now()): Ranked[] {
    const rng = mulberry32(seed);
    const byApp = new Map<string, number[]>();
    for (const c of candidates) {
      if (c.quotaHeadroom !== undefined) {
        byApp.set(c.app, [...(byApp.get(c.app) ?? []), c.quotaHeadroom]);
      }
    }
    const prefs = candidates.map((c) => c.preference ?? 0);
    const pMin = Math.min(...prefs);
    const pSpan = Math.max(...prefs) - pMin || 1;

    return candidates
      .map((c) => {
        const arm = this.arms.get(`${c.id}|${type}`) ?? PRIOR;
        const sample = sampleBeta(arm.a, arm.b, rng);
        const appHeadrooms = byApp.get(c.app);
        // Same-app profiles share the app's mean headroom: quota cannot pick between them.
        const quota = appHeadrooms
          ? appHeadrooms.reduce((a, b) => a + b, 0) / appHeadrooms.length
          : 0.5;
        const pref = ((c.preference ?? 0) - pMin) / pSpan;
        const score = 0.6 * sample + 0.25 * quota + 0.15 * pref;
        const obs = armObservations(arm);
        const why: Reason[] = [
          {
            signal: obs
              ? `${Math.round(armMean(arm) * 100)} % de éxito en ${obs} tareas ${type}`
              : `sin historial en tareas ${type} (exploración)`,
            weight: 0.6 * sample,
          },
          {
            signal: appHeadrooms
              ? `margen de cuota de ${c.app}: ${Math.round(quota * 100)} %`
              : 'cuota no reportada',
            weight: 0.25 * quota,
          },
        ];
        if (c.preference) why.push({ signal: `preferencia ${c.preference}`, weight: 0.15 * pref });
        return { id: c.id, score, why };
      })
      .sort((a, b) => b.score - a.score);
  }

  // ── Learning ─────────────────────────────────────────────────────────────

  private record(q: Question, p: number, y: 0 | 1): void {
    this.history.push({ q, p, y });
    if (this.history.length > HISTORY_MAX)
      this.history.splice(0, this.history.length - HISTORY_MAX);
  }

  /** The user confirmed or corrected the task type. Corrections count double. */
  learnTaskType(s: TaskSignals, label: TaskType): void {
    const predicted = this.taskType(s);
    const corrected = predicted.value !== label;
    this.record('taskType', predicted.p, corrected ? 0 : 1);
    this.taskTypeModel.update(taskFeatures(s), TASK_TYPES.indexOf(label), corrected ? 2 : 1);
  }

  learnComplexity(s: TaskSignals, type: TaskType, userScore: number): void {
    const rule = complexityRules(s).score;
    const cur = this.complexityOffset[type] ?? { sum: 0, n: 0 };
    const delta = Math.max(-4, Math.min(4, userScore - rule));
    this.complexityOffset[type] = { sum: cur.sum + delta, n: cur.n + 1 };
  }

  learnSensitivity(s: TaskSignals, sensitive: boolean): void {
    const d = this.sensitive(s);
    this.record('sensitivity', d.p, d.value === sensitive ? 1 : 0);
    this.sensitivityModel.update(taskFeatures(s), sensitive ? 1 : 0, d.value === sensitive ? 1 : 2);
  }

  learnAcceptance(r: RunSignals, accepted: boolean): void {
    const d = this.meetsCriteria(r);
    this.record('acceptance', d.p, d.value === accepted ? 1 : 0);
    this.acceptanceModel.update(runFeatures(r), accepted ? 1 : 0);
  }

  /** Outcome of a task run by a profile: feeds the profile ranking. */
  recordOutcome(profileId: string, type: TaskType, success: boolean): void {
    const key = `${profileId}|${type}`;
    this.arms.set(key, updateArm(this.arms.get(key) ?? PRIOR, success));
  }

  // ── Introspection & persistence ──────────────────────────────────────────

  stats(): {
    examples: Record<Question, number>;
    brier: number | null;
    byQuestion: Record<Question, number | null>;
    reliability: ReturnType<typeof reliability>;
    profiles: Array<{ key: string; mean: number; n: number }>;
  } {
    const all: Outcome[] = this.history.map((h) => ({ p: h.p, y: h.y }));
    const by = (q: Question) =>
      brier(this.history.filter((h) => h.q === q).map((h) => ({ p: h.p, y: h.y })));
    return {
      examples: {
        taskType: this.taskTypeModel.updates,
        sensitivity: this.sensitivityModel.updates,
        acceptance: this.acceptanceModel.updates,
      },
      brier: brier(all),
      byQuestion: {
        taskType: by('taskType'),
        sensitivity: by('sensitivity'),
        acceptance: by('acceptance'),
      },
      reliability: reliability(all),
      profiles: [...this.arms].map(([key, arm]) => ({
        key,
        mean: armMean(arm),
        n: armObservations(arm),
      })),
    };
  }

  toJSON(): CriterioState {
    return {
      version: 1,
      taskType: this.taskTypeModel.toJSON(),
      sensitivity: this.sensitivityModel.toJSON(),
      acceptance: this.acceptanceModel.toJSON(),
      complexityOffset: this.complexityOffset,
      arms: Object.fromEntries(this.arms),
      history: this.history,
    };
  }
}
