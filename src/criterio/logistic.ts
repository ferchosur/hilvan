import { bucket, type Feature } from './features.js';
import { softmax } from './rules.js';
import type { Reason } from './types.js';

/** Human label for a feature name; structural features are not shown as reasons. */
function readable(name: string): string | null {
  if (name.startsWith('w:') || name.startsWith('r:')) return `"${name.slice(2)}"`;
  if (name.startsWith('b:')) return `"${name.slice(2).replace('_', ' ')}"`;
  if (name.startsWith('diff.') || name.startsWith('state:') || name === 'denials') return name;
  return null;
}

/** Serialized form: only hashed buckets with non-zero weights. */
export interface SoftmaxState {
  classes: number;
  /** bucket → weight per class */
  weights: Record<string, number[]>;
  updates: number;
}

export interface SoftmaxOptions {
  /** Base learning rate; decays with 1/sqrt(t). */
  lr?: number;
  /** L2 regularisation applied to touched weights. */
  l2?: number;
  /** Hard cap on |weight| to bound the influence of any single signal (anti-poisoning). */
  clip?: number;
}

/**
 * Online multinomial logistic regression over hashed sparse features (SGD + L2).
 * Binary questions use 2 classes. Typical update/predict cost: a few microseconds.
 */
export class OnlineSoftmax {
  readonly classes: number;
  private readonly w = new Map<number, Float64Array>();
  private t = 0;
  private readonly lr: number;
  private readonly l2: number;
  private readonly clip: number;

  constructor(classes: number, opts: SoftmaxOptions = {}) {
    this.classes = classes;
    this.lr = opts.lr ?? 0.5;
    this.l2 = opts.l2 ?? 1e-4;
    this.clip = opts.clip ?? 4;
  }

  get updates(): number {
    return this.t;
  }

  private logits(feats: readonly Feature[]): number[] {
    const z = new Array<number>(this.classes).fill(0);
    for (const f of feats) {
      const row = this.w.get(bucket(f.name));
      if (!row) continue;
      for (let c = 0; c < this.classes; c++) z[c] = (z[c] ?? 0) + (row[c] ?? 0) * f.value;
    }
    return z;
  }

  predict(feats: readonly Feature[]): number[] {
    return softmax(this.logits(feats));
  }

  /** Signals that most support class `c`, as readable reasons. */
  explain(feats: readonly Feature[], c: number, top = 4): Reason[] {
    const out: Reason[] = [];
    for (const f of feats) {
      if (f.name === 'bias') continue;
      const row = this.w.get(bucket(f.name));
      if (!row) continue;
      // contribution relative to the mean of the other classes
      let others = 0;
      for (let k = 0; k < this.classes; k++) if (k !== c) others += row[k] ?? 0;
      const contrib = ((row[c] ?? 0) - others / Math.max(1, this.classes - 1)) * f.value;
      const label = readable(f.name);
      if (label && Math.abs(contrib) > 0.05)
        out.push({ signal: `aprendido: ${label}`, weight: contrib });
    }
    return out.sort((a, b) => b.weight - a.weight).slice(0, top);
  }

  /** One SGD step towards label `y`. `weight` scales the step (e.g. explicit corrections count more). */
  update(feats: readonly Feature[], y: number, weight = 1): void {
    this.t++;
    const p = this.predict(feats);
    const eta = (this.lr / Math.sqrt(this.t)) * weight;
    for (const f of feats) {
      const b = bucket(f.name);
      let row = this.w.get(b);
      if (!row) {
        row = new Float64Array(this.classes);
        this.w.set(b, row);
      }
      for (let c = 0; c < this.classes; c++) {
        const grad = ((p[c] ?? 0) - (c === y ? 1 : 0)) * f.value + this.l2 * (row[c] ?? 0);
        const next = (row[c] ?? 0) - eta * grad;
        row[c] = Math.max(-this.clip, Math.min(this.clip, next));
      }
    }
  }

  toJSON(): SoftmaxState {
    const weights: Record<string, number[]> = {};
    for (const [b, row] of this.w) {
      if (row.some((x) => Math.abs(x) > 1e-6))
        weights[b] = Array.from(row, (x) => Math.round(x * 1e5) / 1e5);
    }
    return { classes: this.classes, weights, updates: this.t };
  }

  static fromJSON(s: SoftmaxState, opts: SoftmaxOptions = {}): OnlineSoftmax {
    const m = new OnlineSoftmax(s.classes, opts);
    m.t = s.updates;
    for (const [b, row] of Object.entries(s.weights)) {
      if (row.length === s.classes) m.w.set(Number(b), Float64Array.from(row));
    }
    return m;
  }
}
