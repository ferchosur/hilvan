/**
 * SR-013: local secret detector for briefs, handoff packages and stored run logs.
 * Known token formats plus a high-entropy fallback. No network, no dependencies.
 */

export interface SecretMatch {
  kind: string;
  start: number;
  end: number;
}

const PATTERNS: ReadonlyArray<{ kind: string; re: RegExp }> = [
  {
    kind: 'private-key',
    re: /-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----/g,
  },
  { kind: 'anthropic-key', re: /\bsk-ant-[A-Za-z0-9_-]{20,}/g },
  { kind: 'openai-key', re: /\bsk-(?:proj-|svcacct-|admin-)?[A-Za-z0-9_-]{20,}/g },
  { kind: 'github-token', re: /\b(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{40,})\b/g },
  { kind: 'aws-access-key', re: /\b(?:AKIA|ASIA)[A-Z0-9]{16}\b/g },
  { kind: 'google-api-key', re: /\bAIza[0-9A-Za-z_-]{35}\b/g },
  { kind: 'slack-token', re: /\bxox[abposr]-[A-Za-z0-9-]{10,}/g },
  { kind: 'stripe-key', re: /\b[rs]k_(?:live|test)_[A-Za-z0-9]{20,}/g },
  { kind: 'jwt', re: /\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}/g },
  { kind: 'url-credentials', re: /\b[a-z][a-z0-9+.-]*:\/\/[^\s:/@]+:[^\s@/]{3,}@[^\s]+/gi },
  {
    kind: 'assignment',
    re: /\b(?:password|passwd|pwd|secret|api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret)\b\s*[:=]\s*["']?([^\s"']{8,})/gi,
  },
];

/** Shannon entropy in bits per character. */
export function entropy(s: string): number {
  if (!s) return 0;
  const counts = new Map<string, number>();
  for (const ch of s) counts.set(ch, (counts.get(ch) ?? 0) + 1);
  let h = 0;
  for (const n of counts.values()) {
    const p = n / s.length;
    h -= p * Math.log2(p);
  }
  return h;
}

const CANDIDATE = /[A-Za-z0-9+/_=-]{32,}/g;

function overlaps(m: SecretMatch, list: readonly SecretMatch[]): boolean {
  return list.some((x) => m.start < x.end && x.start < m.end);
}

export function findSecrets(text: string): SecretMatch[] {
  const found: SecretMatch[] = [];
  for (const { kind, re } of PATTERNS) {
    for (const m of text.matchAll(re)) {
      const start = m.index ?? 0;
      const match: SecretMatch = { kind, start, end: start + m[0].length };
      if (!overlaps(match, found)) found.push(match);
    }
  }
  for (const m of text.matchAll(CANDIDATE)) {
    const s = m[0];
    const start = m.index ?? 0;
    const match: SecretMatch = { kind: 'high-entropy', start, end: start + s.length };
    // Mixed character classes and high entropy: unlikely to be a word, path or plain hex hash.
    const classes = [/[a-z]/, /[A-Z]/, /\d/].filter((r) => r.test(s)).length;
    if (classes >= 3 && entropy(s) >= 4.3 && !overlaps(match, found)) found.push(match);
  }
  return found.sort((a, b) => a.start - b.start);
}

/** Replaces each secret with `[REDACTED:<kind>]`. */
export function redactSecrets(text: string): { text: string; matches: SecretMatch[] } {
  const matches = findSecrets(text);
  let out = '';
  let last = 0;
  for (const m of matches) {
    out += `${text.slice(last, m.start)}[REDACTED:${m.kind}]`;
    last = m.end;
  }
  return { text: out + text.slice(last), matches };
}
