/** A past probabilistic prediction and what actually happened. */
export interface Outcome {
  p: number;
  y: 0 | 1;
}

/** Mean squared error of probabilities; 0 is perfect, 0.25 is a coin flip. */
export function brier(outcomes: readonly Outcome[]): number | null {
  if (!outcomes.length) return null;
  return outcomes.reduce((s, o) => s + (o.p - o.y) ** 2, 0) / outcomes.length;
}

export interface ReliabilityBin {
  from: number;
  to: number;
  count: number;
  meanP: number;
  observed: number;
}

/** Reliability curve: in each confidence band, how often the prediction was right. */
export function reliability(outcomes: readonly Outcome[], bins = 5): ReliabilityBin[] {
  const out: ReliabilityBin[] = [];
  for (let i = 0; i < bins; i++) {
    const from = i / bins;
    const to = (i + 1) / bins;
    const inBin = outcomes.filter((o) => o.p >= from && (i === bins - 1 ? o.p <= to : o.p < to));
    out.push({
      from,
      to,
      count: inBin.length,
      meanP: inBin.length ? inBin.reduce((s, o) => s + o.p, 0) / inBin.length : 0,
      observed: inBin.length ? inBin.reduce((s, o) => s + o.y, 0) / inBin.length : 0,
    });
  }
  return out;
}
