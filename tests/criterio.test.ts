import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import {
  CriterioStateSchema,
  loadCriterio,
  MAX_MODEL_BYTES,
  saveCriterio,
} from '../src/config/criterio-store.js';
import {
  brier,
  Criterio,
  disposition,
  mulberry32,
  reliability,
  sampleBeta,
  tokenize,
} from '../src/criterio/index.js';

describe('features', () => {
  it('normalises accents, drops stopwords and stems es/en to shared roots', () => {
    expect(tokenize('Refactorizar la documentación')).toEqual(['refact', 'docume']);
    expect(tokenize('refactor the documentation')).toEqual(['refact', 'docume']);
  });
});

describe('task type (rules, cold start)', () => {
  const c = new Criterio();
  it.each([
    ['Refactorizar el módulo de pagos', 'refactor'],
    ['Corregir bug en el login', 'fix'],
    ['Documentar la API en el README', 'docs'],
    ['Add support for SSH destinations', 'implement'],
    ['Investigar alternativas de UI para terminal', 'research'],
    ['Aumentar la cobertura de tests unitarios', 'test'],
    ['Revisar el PR de autenticación', 'review'],
  ])('%s → %s', (title, expected) => {
    const d = c.taskType({ title });
    expect(d.value).toBe(expected);
    expect(d.source).toBe('rules');
    expect(d.why.length).toBeGreaterThan(0);
  });
});

describe('learning from corrections (AC-022)', () => {
  it('learns a word the rules do not know', () => {
    const c = new Criterio();
    const before = c.taskType({ title: 'Spike de tuberías WSL' });
    expect(before.value).not.toBe('research');
    for (let i = 0; i < 12; i++) {
      c.learnTaskType(
        { title: `Spike de ${['sockets', 'colas', 'caché', 'hilos'][i % 4]}` },
        'research',
      );
      c.learnTaskType(
        { title: `Implementar ${['login', 'logout', 'perfil'][i % 3]}` },
        'implement',
      );
    }
    const after = c.taskType({ title: 'Spike de tuberías WSL' });
    expect(after.value).toBe('research');
    expect(after.source).not.toBe('rules');
    expect(after.why.some((r) => r.signal.includes('spike'))).toBe(true);
  });

  it('does not tilt unrelated tasks after a few examples of one type', () => {
    const c = new Criterio();
    for (const t of ['colas', 'sockets', 'caché'])
      c.learnTaskType({ title: `Spike de ${t}` }, 'research');
    expect(c.taskType({ title: 'Corregir bug en el login' }).value).toBe('fix');
    expect(c.taskType({ title: 'Exportar reporte mensual' }).value).not.toBe('research');
  });

  it('shifts complexity towards user corrections per task type', () => {
    const c = new Criterio();
    const s = { title: 'Ajustar el logo' };
    const base = c.complexity(s, 'implement').value;
    for (let i = 0; i < 5; i++) c.learnComplexity(s, 'implement', 9);
    expect(c.complexity(s, 'implement').value).toBeGreaterThan(base + 2);
  });
});

describe('sensitivity and classification', () => {
  const c = new Criterio();
  it('flags corporate-looking briefs and suggests hardening', () => {
    const s = {
      title: 'Exportar datos de clientes desde Jira interno',
      text: 'Contrato confidencial',
    };
    expect(c.sensitive(s).value).toBe(true);
    expect(c.suggestClassification(s, 'personal').value).toBe('corporate');
  });

  it('never suggests a less strict classification', () => {
    const s = { title: 'Exportar datos de clientes desde Jira interno' };
    expect(c.suggestClassification(s, 'confidential').value).toBeNull();
    expect(
      c.suggestClassification({ title: 'Cambiar color del botón' }, 'personal').value,
    ).toBeNull();
  });
});

describe('diff risk', () => {
  const c = new Criterio();
  it('rates secrets, CI and large untested changes as risky', () => {
    const risky = c.diffRisk({
      filesChanged: 12,
      linesAdded: 600,
      linesDeleted: 900,
      paths: ['.env', '.github/workflows/ci.yml', 'src/auth/session.ts'],
    });
    const calm = c.diffRisk({
      filesChanged: 1,
      linesAdded: 5,
      linesDeleted: 1,
      paths: ['README.md'],
    });
    expect(risky.value).toBeGreaterThanOrEqual(8);
    expect(calm.value).toBeLessThanOrEqual(3);
  });
});

describe('acceptance', () => {
  const c = new Criterio();
  it('uses the final state and the result wording', () => {
    expect(c.meetsCriteria({ state: 'blocked' }).value).toBe(false);
    expect(c.meetsCriteria({ state: 'completed', resultText: 'Listo, tests en verde' }).value).toBe(
      true,
    );
    const doubtful = c.meetsCriteria({
      state: 'completed',
      resultText: 'I could not run the tests',
      permissionDenials: 2,
    });
    expect(doubtful.value).toBe(false);
  });
});

describe('thresholds', () => {
  it('never automates sending data or switching profile', () => {
    expect(disposition(0.99, 'send_data')).toBe('confirm');
    expect(disposition(0.99, 'switch_profile')).toBe('confirm');
    expect(disposition(0.99, 'label')).toBe('auto');
    expect(disposition(0.7, 'label')).toBe('confirm');
    expect(disposition(0.4, 'label')).toBe('confirm');
    expect(disposition(0.4, 'suggest_review')).toBe('escalate');
    expect(disposition(0.99, 'harden_classification')).toBe('confirm');
    expect(disposition(0.99, 'label', 9)).toBe('escalate');
  });
});

describe('profile ranking', () => {
  it('prefers the profile with a better history for the task type', () => {
    const c = new Criterio();
    for (let i = 0; i < 15; i++) {
      c.recordOutcome('codex', 'refactor', true);
      c.recordOutcome('claude', 'refactor', i % 3 === 0);
    }
    const ranked = c.rankProfiles(
      [
        { id: 'claude', app: 'claude-code' },
        { id: 'codex', app: 'codex' },
      ],
      'refactor',
      42,
    );
    expect(ranked[0]?.id).toBe('codex');
    expect(ranked[0]?.why[0]?.signal).toMatch(/% de éxito en 15 tareas refactor/);
  });

  it('is deterministic for a given seed', () => {
    const c = new Criterio();
    const cands = [
      { id: 'a', app: 'x' },
      { id: 'b', app: 'y' },
      { id: 'c', app: 'z' },
    ];
    expect(c.rankProfiles(cands, 'fix', 7)).toEqual(c.rankProfiles(cands, 'fix', 7));
  });

  it('does not let quota headroom separate profiles of the same app (FR-012)', () => {
    const c = new Criterio();
    const ranked = c.rankProfiles(
      [
        { id: 'claude-a', app: 'claude-code', quotaHeadroom: 0.95 },
        { id: 'claude-b', app: 'claude-code', quotaHeadroom: 0.05 },
      ],
      'implement',
      1,
    );
    const quota = ranked.map((r) => r.why.find((w) => w.signal.startsWith('margen'))?.weight);
    expect(quota[0]).toBe(quota[1]);
  });

  it('lets quota headroom inform choices between different apps', () => {
    const c = new Criterio();
    const ranked = c.rankProfiles(
      [
        { id: 'claude', app: 'claude-code', quotaHeadroom: 0.05 },
        { id: 'codex', app: 'codex', quotaHeadroom: 0.95 },
      ],
      'implement',
      3,
    );
    const q = Object.fromEntries(
      ranked.map((r) => [r.id, r.why.find((w) => w.signal.startsWith('margen'))?.weight ?? 0]),
    );
    expect(q.codex).toBeGreaterThan(q.claude ?? 0);
  });
});

describe('statistics', () => {
  it('computes Brier score and reliability', () => {
    expect(brier([])).toBeNull();
    expect(
      brier([
        { p: 1, y: 1 },
        { p: 0, y: 0 },
      ]),
    ).toBe(0);
    expect(brier([{ p: 0.5, y: 1 }])).toBe(0.25);
    expect(reliability([{ p: 0.9, y: 1 }], 5)[4]?.count).toBe(1);
  });

  it('samples Beta distributions around their mean', () => {
    const rng = mulberry32(1);
    const xs = Array.from({ length: 4000 }, () => sampleBeta(8, 2, rng));
    const mean = xs.reduce((a, b) => a + b, 0) / xs.length;
    expect(mean).toBeGreaterThan(0.77);
    expect(mean).toBeLessThan(0.83);
  });
});

describe('performance (NFR-013)', () => {
  it('answers a decision in well under 5 ms', () => {
    const c = new Criterio();
    for (let i = 0; i < 50; i++) c.learnTaskType({ title: `Implementar cosa ${i}` }, 'implement');
    const text =
      'Refactorizar el adaptador de Codex para normalizar eventos JSONL y añadir pruebas';
    const n = 2000;
    const t0 = performance.now();
    for (let i = 0; i < n; i++) c.taskType({ title: `${text} ${i}` });
    const perDecision = (performance.now() - t0) / n;
    expect(perDecision).toBeLessThan(5);
  });
});

describe('persistence', () => {
  let dir: string;
  beforeEach(async () => {
    dir = await mkdtemp(path.join(os.tmpdir(), 'hilvan-crit-'));
  });
  afterEach(async () => {
    await rm(dir, { recursive: true, force: true });
  });

  it('round-trips a trained model without storing text', async () => {
    const c = new Criterio();
    c.learnTaskType({ title: 'Spike secreto proyecto Atlas' }, 'research');
    c.recordOutcome('p1', 'research', true);
    const file = path.join(dir, 'criterio', 'model.json');
    await saveCriterio(file, c);
    const { engine, warning } = await loadCriterio(file);
    expect(warning).toBeUndefined();
    expect(engine.toJSON()).toEqual(c.toJSON());
    const raw = JSON.stringify(c.toJSON());
    expect(raw).not.toMatch(/atlas|secret|spike/i);
    expect(CriterioStateSchema.safeParse(c.toJSON()).success).toBe(true);
  });

  it('falls back to rules when the file is corrupted or oversized', async () => {
    const bad = path.join(dir, 'bad.json');
    await writeFile(bad, '{"version":1,"taskType":42}');
    const r1 = await loadCriterio(bad);
    expect(r1.warning).toMatch(/invalid/);
    const big = path.join(dir, 'big.json');
    await writeFile(big, ' '.repeat(MAX_MODEL_BYTES + 1));
    expect((await loadCriterio(big)).warning).toMatch(/too large/);
    expect((await loadCriterio(path.join(dir, 'missing.json'))).warning).toBeUndefined();
  });
});
