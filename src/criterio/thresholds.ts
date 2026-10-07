import type { Action, Disposition } from './types.js';

export interface Thresholds {
  /** At or above: benign actions may run automatically. */
  auto: number;
  /** Below: escalate to the advisor or the user. */
  escalate: number;
  /** Risk score (2..10) at or above which any action escalates. */
  riskEscalate: number;
}

export const DEFAULT_THRESHOLDS: Thresholds = { auto: 0.85, escalate: 0.55, riskEscalate: 7.5 };

/** Actions that move data between identities or change who runs a task (D-009, FR-020). */
const SENSITIVE: ReadonlySet<Action> = new Set(['send_data', 'switch_profile']);
/** Actions that are visible and reversible: low confidence just asks the user, it never escalates. */
const BENIGN: ReadonlySet<Action> = new Set(['label', 'order']);
/** Actions the user must always see, even when confident (classification changes eligibility). */
const ALWAYS_SHOWN: ReadonlySet<Action> = new Set(['harden_classification']);

/**
 * Maps confidence and risk to what Hilvan may do on its own.
 * Sensitive actions are never `auto`, whatever the confidence.
 */
export function disposition(
  p: number,
  action: Action,
  risk = 0,
  t: Thresholds = DEFAULT_THRESHOLDS,
): Disposition {
  if (risk >= t.riskEscalate) return 'escalate';
  if (p < t.escalate) return BENIGN.has(action) ? 'confirm' : 'escalate';
  if (SENSITIVE.has(action) || ALWAYS_SHOWN.has(action)) return 'confirm';
  return p >= t.auto ? 'auto' : 'confirm';
}
