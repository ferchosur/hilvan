import { isSensitivePath, isTestPath, stem, tokenize } from './features.js';
import type { DiffSignals, Reason, RunSignals, TaskSignals, TaskType } from './types.js';
import { TASK_TYPES } from './types.js';

/** Seed vocabulary (es/en). Stemmed at load so both languages share roots. */
const LEXICON: Record<TaskType, string> = {
  implement:
    'implementar implement anadir agregar add crear create nuevo nueva new feature funcionalidad soporte support construir build integrar integrate habilitar enable endpoint comando command',
  fix: 'corregir fix arreglar bug error falla fallo crash roto broken regresion regression solucionar resolve problema issue excepcion exception',
  refactor:
    'refactor refactorizar renombrar rename extraer extract limpiar cleanup simplificar simplify reorganizar reorganize restructure mover move duplicado duplicate modularizar desacoplar decouple',
  test: 'test tests prueba pruebas testing cobertura coverage unitario unit e2e fixture mock assert',
  review:
    'revisar review auditar audit inspeccionar inspect evaluar evaluate verificar verify validar validate feedback observaciones',
  research:
    'investigar research comparar compare explorar explore analizar analyze opciones options alternativas alternatives benchmark estudio study viabilidad feasibility encuesta',
  docs: 'documentar document documentacion documentation readme docs guia guide manual changelog comentarios comments tutorial explicar explain',
};

const STEMS: ReadonlyMap<string, TaskType[]> = (() => {
  const m = new Map<string, TaskType[]>();
  for (const t of TASK_TYPES) {
    for (const w of LEXICON[t].split(' ')) {
      const s = stem(w);
      m.set(s, [...new Set([...(m.get(s) ?? []), t])]);
    }
  }
  return m;
})();

export function softmax(xs: readonly number[]): number[] {
  const max = Math.max(...xs);
  const ex = xs.map((x) => Math.exp(x - max));
  const sum = ex.reduce((a, b) => a + b, 0);
  return ex.map((e) => e / sum);
}

/** Rule prior over task types: keyword hits through a tempered softmax. */
export function taskTypeRules(s: TaskSignals): { probs: number[]; why: Reason[] } {
  const hits = new Array<number>(TASK_TYPES.length).fill(0);
  const why: Reason[] = [];
  const titleToks = new Set(tokenize(s.title ?? ''));
  for (const tok of new Set(tokenize(`${s.title ?? ''} ${s.text ?? ''}`))) {
    const types = STEMS.get(tok);
    if (!types) continue;
    const w = titleToks.has(tok) ? 2 : 1; // the title says more than the body
    for (const t of types) hits[TASK_TYPES.indexOf(t)] = (hits[TASK_TYPES.indexOf(t)] ?? 0) + w;
    why.push({ signal: `palabra "${tok}" → ${types.join('/')}`, weight: w });
  }
  hits[0] = (hits[0] ?? 0) + 0.3; // mild prior: unlabeled work is usually implementation
  return { probs: softmax(hits.map((h) => h * 1.2)), why: why.sort((a, b) => b.weight - a.weight) };
}

const HEAVY =
  'arquitectura architecture migracion migration seguridad security concurrencia concurrency paralelo parallel distribuido distributed rendimiento performance multiples multiple varios several sistema system rediseno redesign protocolo protocol database';

const HEAVY_STEMS = new Set(HEAVY.split(' ').map(stem));

/** Complexity on 2..10 from text length, acceptance criteria and heavy-topic words. */
export function complexityRules(s: TaskSignals): { score: number; why: Reason[] } {
  const toks = tokenize(`${s.title ?? ''} ${s.text ?? ''}`);
  const heavy = [...new Set(toks.filter((t) => HEAVY_STEMS.has(t)))];
  const why: Reason[] = [];
  let score = 3;
  const lenBoost = Math.min(2.5, Math.log2(1 + toks.length / 15));
  score += lenBoost;
  if (lenBoost > 0.5) why.push({ signal: `brief de ${toks.length} palabras`, weight: lenBoost });
  if (s.criteriaCount) {
    const c = Math.min(2, s.criteriaCount * 0.4);
    score += c;
    why.push({ signal: `${s.criteriaCount} criterios de aceptación`, weight: c });
  }
  if (heavy.length) {
    const h = Math.min(3, heavy.length * 1);
    score += h;
    why.push({ signal: `temas exigentes: ${heavy.join(', ')}`, weight: h });
  }
  return { score: clampScore(score), why: why.sort((a, b) => b.weight - a.weight) };
}

/** Risk of a diff on 2..10: size, sensitive paths, deletions, untested changes. */
export function diffRiskRules(d: DiffSignals): { score: number; why: Reason[] } {
  const why: Reason[] = [];
  const lines = d.linesAdded + d.linesDeleted;
  let score = 2;
  const size = Math.min(3, Math.log10(1 + lines));
  score += size;
  why.push({ signal: `${lines} líneas cambiadas en ${d.filesChanged} archivos`, weight: size });
  const sensitive = d.paths.filter(isSensitivePath);
  if (sensitive.length) {
    const w = Math.min(3, 1.5 + sensitive.length * 0.5);
    score += w;
    why.push({ signal: `rutas sensibles: ${sensitive.slice(0, 3).join(', ')}`, weight: w });
  }
  if (lines > 0 && d.linesDeleted / lines > 0.6 && lines > 30) {
    score += 1.5;
    why.push({ signal: 'predominan las eliminaciones', weight: 1.5 });
  }
  const codeFiles = d.paths.filter((p) => !isTestPath(p) && !/\.(md|txt|rst)$/i.test(p));
  if (codeFiles.length > 0 && !d.paths.some(isTestPath)) {
    score += 1;
    why.push({ signal: 'cambia código sin tocar tests', weight: 1 });
  }
  return { score: clampScore(score), why: why.sort((a, b) => b.weight - a.weight) };
}

const CORPORATE =
  'confidencial confidential interno internal cliente clientes customer customers jira confluence salesforce sap nomina payroll contrato contract nda propietario proprietary empresa company corporativo corporate produccion production financiero financial';
const CORP_STEMS = new Set(CORPORATE.split(' ').map(stem));
const PII = [
  /\b\d{4}[ -]?\d{4}[ -]?\d{4}[ -]?\d{4}\b/, // card-like number
  /\b[A-Z]{2}\d{2}[A-Z0-9]{11,30}\b/, // IBAN-like
  /\b[\w.+-]+@[\w-]+\.[\w.]+\b/, // email
];

/** Signals that the brief may carry corporate or personal data (only used to harden, never relax). */
export function sensitivityRules(s: TaskSignals): { p: number; why: Reason[] } {
  const text = `${s.title ?? ''} ${s.text ?? ''}`;
  const hits = [...new Set(tokenize(text).filter((t) => CORP_STEMS.has(t)))];
  const why: Reason[] = hits.map((h) => ({ signal: `palabra "${h}"`, weight: 1 }));
  let z = -2 + hits.length * 1.2;
  for (const re of PII) {
    if (re.test(text)) {
      z += 2;
      why.push({ signal: 'posible dato personal o financiero en el texto', weight: 2 });
    }
  }
  return { p: 1 / (1 + Math.exp(-z)), why };
}

const FAILURE_HINTS =
  /\b(no pude|no fue posible|could not|couldn't|unable to|not implemented|todo|fixme|failed|fall[oó]|error)\b/i;

/** Did a run likely satisfy the brief? Rule prior from final state and result wording. */
export function acceptanceRules(r: RunSignals): { p: number; why: Reason[] } {
  const why: Reason[] = [];
  if (r.state !== 'completed') {
    return { p: 0.05, why: [{ signal: `la ejecución terminó en ${r.state}`, weight: -3 }] };
  }
  let z = 1.2;
  why.push({ signal: 'la ejecución terminó correctamente', weight: 1.2 });
  if (r.permissionDenials) {
    z -= Math.min(2, r.permissionDenials * 0.7);
    why.push({ signal: `${r.permissionDenials} permisos denegados`, weight: -0.7 });
  }
  if (r.resultText && FAILURE_HINTS.test(r.resultText)) {
    z -= 1.5;
    why.push({ signal: 'el resumen menciona problemas o pendientes', weight: -1.5 });
  }
  if (r.diff && r.diff.filesChanged === 0 && (r.criteriaCount ?? 0) > 0) {
    z -= 1;
    why.push({ signal: 'no hubo cambios pese a tener criterios', weight: -1 });
  }
  return {
    p: 1 / (1 + Math.exp(-z)),
    why: why.sort((a, b) => Math.abs(b.weight) - Math.abs(a.weight)),
  };
}

export function clampScore(x: number): number {
  return Math.round(Math.min(10, Math.max(2, x)) * 10) / 10;
}
