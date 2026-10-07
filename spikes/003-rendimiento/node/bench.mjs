// Spike 003 — Node.js slice of Hilvan (stdlib only).
import { spawn } from 'node:child_process';
import { createReadStream } from 'node:fs';
import { createInterface } from 'node:readline';

const TEXTS = [
  'Refactor the Codex adapter to normalise JSONL events and add tests',
  'Fix bug in login flow when the token expires',
  'Document the API in the README with examples',
  'Investigate terminal UI alternatives for low resource machines',
  'Add support for SSH destinations in the launcher',
  'Review the authentication pull request for security issues',
  'Increase unit test coverage of the policy evaluator',
  'Implement export of task history to CSV files',
];
const K = 7;
const BUCKETS = 65536;

function fnv1a(s) {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}
function tokenize(t) {
  const out = [];
  for (const w of t.toLowerCase().split(/[^a-z0-9]+/)) if (w.length >= 2) out.push(w.length > 6 ? w.slice(0, 6) : w);
  return out;
}
function features(t) {
  const toks = tokenize(t);
  const f = [];
  for (let i = 0; i < toks.length; i++) {
    f.push(`w:${toks[i]}`);
    if (i + 1 < toks.length) f.push(`b:${toks[i]}_${toks[i + 1]}`);
  }
  return f;
}
function weightsFor(b) {
  const w = new Float64Array(K);
  for (let c = 0; c < K; c++) w[c] = (((b * (c + 1) * 2654435761) >>> 0) % 1000) / 1000 - 0.5;
  return w;
}
function criterio(n) {
  const W = new Map();
  for (const t of TEXTS) for (const f of features(t)) { const b = fnv1a(f) % BUCKETS; W.set(b, weightsFor(b)); }
  let check = 0;
  for (let i = 0; i < n; i++) {
    const z = new Float64Array(K);
    for (const f of features(`${TEXTS[i % 8]} ${i % 100}`)) {
      const w = W.get(fnv1a(f) % BUCKETS);
      if (w) for (let c = 0; c < K; c++) z[c] += w[c];
    }
    let m = -Infinity; for (const v of z) m = Math.max(m, v);
    let s = 0; const p = Array.from(z, (v) => { const e = Math.exp(v - m); s += e; return e; });
    let best = 0; for (let c = 1; c < K; c++) if (p[c] > p[best]) best = c;
    check += best;
  }
  console.log(`criterio n=${n} check=${check}`);
}
function newStats() { return { lines: 0, session: 0, text: 0, limits: 0, results: 0, tokens: 0 }; }
function handle(st, line) {
  st.lines++;
  const e = JSON.parse(line);
  switch (e.type) {
    case 'system': if (e.subtype === 'init') st.session++; break;
    case 'assistant': for (const c of e.message?.content ?? []) if (c.type === 'text') st.text++; break;
    case 'rate_limit_event': st.limits++; break;
    case 'result': st.results++; st.tokens += (e.usage?.input_tokens ?? 0) + (e.usage?.output_tokens ?? 0); break;
  }
}
function report(st) { console.log(`lines=${st.lines} session=${st.session} text=${st.text} limits=${st.limits} results=${st.results} tokens=${st.tokens}`); }
async function readLines(stream) {
  const st = newStats();
  for await (const line of createInterface({ input: stream, crlfDelay: Infinity })) if (line) handle(st, line);
  report(st);
}

const [mode, arg] = process.argv.slice(2);
if (mode === 'version') console.log('hilvan-node 0.0.0');
else if (mode === 'criterio') criterio(Number(arg));
else if (mode === 'jsonl') await readLines(createReadStream(arg));
else if (mode === 'spawn') await readLines(spawn('cat', [arg], { stdio: ['ignore', 'pipe', 'inherit'] }).stdout);
else { console.error('usage: version | criterio N | jsonl FILE | spawn FILE'); process.exit(2); }
