// Spike 006 (desechable): genera casos dorados con la implementación TypeScript real.
// Uso: npx tsx spikes/006-paridad-go/gen.ts
import { writeFileSync } from 'node:fs';
import { canonical } from '../../src/store/audit.js';
import { fnv1a, tokenize } from '../../src/criterio/features.js';

// PRNG determinista (mulberry32) para que los casos sean reproducibles.
let seed = 0x5eed;
const rnd = () => {
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};

// 1) Números: bordes conocidos + dobles aleatorios en muchas magnitudes.
const nums: number[] = [0, -0, 1, -1, 0.1, 0.2, 0.1 + 0.2, 1e21, 1e20, 123456789012345680000, 1e-6, 1e-7,
  1.5e-7, 5e-324, 1.7976931348623157e308, 2 ** 53, 2 ** 53 + 2, 0.000001, 123e-20, 100, 1e100, -1e-100, 3.14159];
for (let i = 0; i < 20000; i++) {
  const e = Math.floor(rnd() * 80) - 40;
  nums.push((rnd() - 0.5) * 10 ** e);
  const buf = new DataView(new ArrayBuffer(8));
  buf.setUint32(0, (rnd() * 2 ** 32) >>> 0);
  buf.setUint32(4, (rnd() * 2 ** 32) >>> 0);
  const f = buf.getFloat64(0);
  if (Number.isFinite(f)) nums.push(f);
}
const bits = (n: number) => {
  const v = new DataView(new ArrayBuffer(8));
  v.setFloat64(0, n);
  return v.getBigUint64(0).toString(16).padStart(16, '0');
};
writeFileSync(new URL('testdata/numbers.jsonl', import.meta.url),
  nums.map((n) => JSON.stringify({ bits: bits(n), out: canonical(n) })).join('\n') + '\n');

// 2) Cadenas y objetos: escapes, controles, Unicode, claves desordenadas, anidados.
const strings = ['', 'a', '"', '\\', '\b\f\n\r\t', '\u0000\u001f\u007f', '<script>&amp;</script>', '  ',
  'ñandú café', '日本語', '😀 emoji', 'mixed "q" \\ \n end', 'tab\tsep'];
for (let i = 0; i < 2000; i++) {
  let s = '';
  const len = Math.floor(rnd() * 12);
  for (let j = 0; j < len; j++) {
    const r = rnd();
    s += r < 0.3 ? String.fromCharCode(Math.floor(rnd() * 0x80))
      : r < 0.6 ? String.fromCharCode(0xa0 + Math.floor(rnd() * 0x700))
      : r < 0.9 ? String.fromCharCode(0x3000 + Math.floor(rnd() * 0x5000))
      : String.fromCodePoint(0x1f300 + Math.floor(rnd() * 0x300));
  }
  strings.push(s);
}
const objects = [
  { ts: '2026-10-07T00:00:00.000Z', type: 'run.started', data: { z: 1, a: [1, 'x', null, true], m: { b: 0.5, a: -0 } } },
  { b: { d: { f: [], e: {} }, c: '<&>' }, a: 1e21, 'á': 1, A: 2, _: 3 },
  ...strings.slice(0, 200).map((s, i) => ({ [s]: s, n: nums[i], list: [s, nums[i]] })),
];
writeFileSync(new URL('testdata/canonical.jsonl', import.meta.url),
  [...strings, ...objects].map((v) => JSON.stringify({ in: v, out: canonical(v) })).join('\n') + '\n');

// 3) Criterio: fnv1a sobre unidades UTF-16 y tokenize (minúsculas, NFD, sin marcas, stopwords, raíz).
const texts = [...strings, 'Refactorizar el módulo de autenticación', 'Añadir TESTS para la migración',
  'İstanbul ǅ ﬁx Kelvin K ß', 'Arreglar bug #123 en api/v2', 'documentation documentar DOCUMENTO'];
writeFileSync(new URL('testdata/criterio.jsonl', import.meta.url),
  texts.map((t) => JSON.stringify({ text: t, fnv: fnv1a(t), tokens: tokenize(t) })).join('\n') + '\n');
console.log(`numbers=${nums.length} canonical=${strings.length + objects.length} criterio=${texts.length}`);
