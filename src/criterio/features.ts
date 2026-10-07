import type { DiffSignals, TaskSignals } from './types.js';

/** A sparse feature: readable name (only kept in memory, for explanations) and value. */
export interface Feature {
  name: string;
  value: number;
}

/** Number of hash buckets. Models store weights per bucket, never the original text. */
export const BUCKETS = 1 << 16;

/** FNV-1a 32-bit hash. Deterministic across platforms and runs. */
export function fnv1a(s: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

export function bucket(name: string): number {
  return fnv1a(name) % BUCKETS;
}

const STOPWORDS = new Set(
  (
    'a al algo and are as at be by con de del el en es esta este for from ha in is it la las lo los ' +
    'of on or para por que se sin su the to un una y que we you our nuestro'
  ).split(' '),
);

const STEM_LEN = 6;

/** Lowercase, strip diacritics, split on non-alphanumerics, drop stopwords, prefix-stem. */
export function tokenize(text: string): string[] {
  const out: string[] = [];
  const norm = text.toLowerCase().normalize('NFD').replace(/\p{M}/gu, '');
  for (const raw of norm.split(/[^a-z0-9]+/)) {
    if (raw.length < 2 || STOPWORDS.has(raw)) continue;
    out.push(stem(raw));
  }
  return out;
}

/** Cheap language-agnostic stemmer: refactorizar/refactor → refact, documentar/documentation → docume. */
export function stem(word: string): string {
  return word.length > STEM_LEN ? word.slice(0, STEM_LEN) : word;
}

const SENSITIVE_PATH =
  /(^|\/)(\.env|.*secret.*|.*credential.*|.*\.pem|.*\.key|id_rsa.*|\.github\/workflows\/.*|.*migrations?\/.*|.*auth.*|.*security.*|package\.json|package-lock\.json|pnpm-lock\.yaml|yarn\.lock|go\.(mod|sum)|cargo\.(toml|lock)|requirements.*\.txt|dockerfile|.*\.tf)$/i;
const TEST_PATH = /(^|\/)(tests?|__tests__|spec)\/|\.(test|spec)\.[a-z0-9]+$/i;

export function isSensitivePath(p: string): boolean {
  return SENSITIVE_PATH.test(p.replaceAll('\\', '/'));
}

export function isTestPath(p: string): boolean {
  return TEST_PATH.test(p.replaceAll('\\', '/'));
}

function bin(name: string, v: number, edges: readonly number[]): Feature {
  let i = 0;
  while (i < edges.length && v > (edges[i] ?? Number.POSITIVE_INFINITY)) i++;
  return { name: `${name}:b${i}`, value: 1 };
}

export function diffFeatures(d: DiffSignals): Feature[] {
  const lines = d.linesAdded + d.linesDeleted;
  const sensitive = d.paths.filter(isSensitivePath).length;
  const tests = d.paths.filter(isTestPath).length;
  const feats: Feature[] = [
    bin('diff.files', d.filesChanged, [1, 3, 10, 30]),
    bin('diff.lines', lines, [20, 100, 400, 1500]),
    { name: 'diff.del_ratio', value: lines ? d.linesDeleted / lines : 0 },
  ];
  if (sensitive) feats.push({ name: 'diff.sensitive', value: Math.min(sensitive, 5) / 5 });
  if (tests) feats.push({ name: 'diff.tests', value: 1 });
  if (!tests && d.filesChanged > 0) feats.push({ name: 'diff.no_tests', value: 1 });
  return feats;
}

/**
 * Bag of stemmed words and bigrams plus diff signals. Duplicate words count once.
 * No bias or length features: class priors come from the rules, so a handful of
 * examples of one type cannot tilt every other task towards it.
 */
export function taskFeatures(s: TaskSignals): Feature[] {
  const text = `${s.title ?? ''} ${s.title ?? ''} ${s.text ?? ''}`;
  const toks = tokenize(text);
  const seen = new Set<string>();
  const feats: Feature[] = [];
  for (let i = 0; i < toks.length; i++) {
    const w = `w:${toks[i]}`;
    if (!seen.has(w)) {
      seen.add(w);
      feats.push({ name: w, value: 1 });
    }
    if (i + 1 < toks.length) {
      const bg = `b:${toks[i]}_${toks[i + 1]}`;
      if (!seen.has(bg)) {
        seen.add(bg);
        feats.push({ name: bg, value: 0.5 });
      }
    }
  }
  if (s.criteriaCount !== undefined) feats.push(bin('criteria', s.criteriaCount, [1, 3, 6]));
  if (s.diff) feats.push(...diffFeatures(s.diff));
  return feats;
}
