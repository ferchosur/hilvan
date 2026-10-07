/** Seeded PRNG (mulberry32) so recommendations are reproducible in tests. */
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function normal(rng: () => number): number {
  // Box–Muller
  const u = Math.max(rng(), 1e-12);
  const v = rng();
  return Math.sqrt(-2 * Math.log(u)) * Math.cos(2 * Math.PI * v);
}

/** Gamma(k, 1) sample, Marsaglia–Tsang; valid for any k > 0. */
export function sampleGamma(k: number, rng: () => number): number {
  if (k < 1) return sampleGamma(k + 1, rng) * rng() ** (1 / k);
  const d = k - 1 / 3;
  const c = 1 / Math.sqrt(9 * d);
  for (;;) {
    let x: number;
    let v: number;
    do {
      x = normal(rng);
      v = 1 + c * x;
    } while (v <= 0);
    v = v * v * v;
    const u = rng();
    if (u < 1 - 0.0331 * x ** 4 || Math.log(u) < 0.5 * x * x + d * (1 - v + Math.log(v))) {
      return d * v;
    }
  }
}

export function sampleBeta(a: number, b: number, rng: () => number): number {
  const x = sampleGamma(a, rng);
  const y = sampleGamma(b, rng);
  return x / (x + y);
}

/** Success/failure counts for one (profile, task type) arm, with a uniform Beta(1,1) prior. */
export interface BetaArm {
  a: number;
  b: number;
}

export const PRIOR: BetaArm = { a: 1, b: 1 };

export function updateArm(arm: BetaArm, success: boolean, weight = 1): BetaArm {
  return success ? { a: arm.a + weight, b: arm.b } : { a: arm.a, b: arm.b + weight };
}

export function armMean(arm: BetaArm): number {
  return arm.a / (arm.a + arm.b);
}

export function armObservations(arm: BetaArm): number {
  return arm.a + arm.b - PRIOR.a - PRIOR.b;
}
