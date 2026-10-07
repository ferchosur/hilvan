/**
 * Criterio — local typed decisions (ADR-0011, FR-020).
 * Pure module: no I/O, no network, no dependencies.
 */

export const TASK_TYPES = [
  'implement',
  'fix',
  'refactor',
  'test',
  'review',
  'research',
  'docs',
] as const;
export type TaskType = (typeof TASK_TYPES)[number];

/** A signal that pushed the decision, with its signed contribution. */
export interface Reason {
  signal: string;
  weight: number;
}

export type DecisionSource = 'rules' | 'model' | 'blend';

export interface Decision<T> {
  value: T;
  /** Confidence in [0, 1]. For `score` questions it is the confidence in the score band. */
  p: number;
  /** Most influential signals, strongest first. */
  why: Reason[];
  source: DecisionSource;
}

/** What a decision would trigger; sensitive actions always need human confirmation. */
export type Action =
  | 'label' // tag a task (type, complexity)
  | 'order' // sort options shown to the user
  | 'suggest_review' // propose an advisor/cross review
  | 'harden_classification' // propose a stricter data classification
  | 'send_data' // hand context to another profile
  | 'switch_profile'; // change the profile that runs a task

export type Disposition = 'auto' | 'confirm' | 'escalate';

/** Text and repository facts about a task. Everything optional: Criterio works with what it has. */
export interface TaskSignals {
  title?: string;
  text?: string;
  criteriaCount?: number;
  /** Diff facts, when a run produced changes. */
  diff?: DiffSignals;
}

export interface DiffSignals {
  filesChanged: number;
  linesAdded: number;
  linesDeleted: number;
  /** Paths touched, used for sensitive-path and test detection. */
  paths: readonly string[];
}

/** Facts about a finished run, for "did it meet the brief?" questions. */
export interface RunSignals {
  state: 'completed' | 'failed' | 'blocked' | 'cancelled' | 'uncertain';
  resultText?: string;
  permissionDenials?: number;
  criteriaCount?: number;
  diff?: DiffSignals;
}
