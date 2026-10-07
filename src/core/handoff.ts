import { createHash } from 'node:crypto';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import { newHandoffId } from '../domain/ids.js';
import type { Profile, RunSummary, Task } from '../domain/index.js';
import type { EffectivePolicy } from '../domain/policy.js';
import { defaultGit, type Git } from '../gitops/index.js';
import { type Evaluation, evaluate } from '../policy/evaluate.js';
import { writeFileAtomic } from '../security/fs.js';
import { findSecrets, redactSecrets, type SecretMatch } from '../security/secrets.js';
import type { ProjectStore } from '../store/project.js';

/**
 * M4 — continuity package between profiles (FR-003, FR-004, FR-005, NFR-002, NFR-003).
 * Built mechanically from what Hilvan observed, so it works even when the source profile
 * is blocked by a usage limit. Agent claims are kept apart as *unverified*.
 */

export class HandoffError extends Error {
  override name = 'HandoffError';
}

export interface HandoffDraft {
  id: string;
  task: Task;
  from: string;
  to: Profile;
  evaluation: Evaluation;
  body: string;
  /** Secrets found (and already redacted) while building the draft. */
  secrets: SecretMatch[];
}

function section(title: string, lines: readonly string[]): string {
  return `## ${title}\n\n${lines.length ? lines.join('\n') : '_(sin datos)_'}\n`;
}

function runLine(r: RunSummary): string {
  const bits = [
    `${r.startedAt.slice(0, 16).replace('T', ' ')} · ${r.profileId} · ${r.state}`,
    r.blockedReason ? `(${r.blockedReason})` : '',
    r.diff ? `· ${r.diff.filesChanged} archivos +${r.diff.linesAdded} −${r.diff.linesDeleted}` : '',
    r.risk !== undefined ? `· riesgo ${r.risk}/10` : '',
  ];
  return `- ${bits.filter(Boolean).join(' ')}`;
}

async function gitFacts(
  store: ProjectStore,
  task: Task,
  git: Git,
): Promise<{ commits: string[]; files: string[] }> {
  if (!task.branch || !task.baseBranch) return { commits: [], files: [] };
  const log = await git(
    ['log', '--oneline', '--no-decorate', '-n', '30', `${task.baseBranch}..${task.branch}`],
    store.root,
  );
  const files = await git(
    ['diff', '--name-status', `${task.baseBranch}...${task.branch}`],
    store.root,
  );
  return {
    commits: log.code === 0 ? log.stdout.trim().split('\n').filter(Boolean) : [],
    files: files.code === 0 ? files.stdout.trim().split('\n').filter(Boolean).slice(0, 200) : [],
  };
}

/** Builds the draft package and evaluates the transfer against the policy (eligibility first). */
export async function draftHandoff(
  store: ProjectStore,
  taskId: string,
  to: Profile,
  fromProfile: Profile | undefined,
  policy: EffectivePolicy,
  opts: { git?: Git } = {},
): Promise<HandoffDraft> {
  const task = await store.loadTask(taskId);
  if (!task.profileId)
    throw new HandoffError('the task has not been assigned to a profile yet; just run it');
  if (task.profileId === to.id) throw new HandoffError(`the task already belongs to ${to.id}`);
  if (task.state === 'running') throw new HandoffError('the task is running; cancel it first');
  if (task.state === 'accepted' || task.state === 'discarded')
    throw new HandoffError(`the task is ${task.state}`);

  const evaluation = evaluate(policy, task, to, fromProfile);
  const brief = await store.brief(taskId);
  const facts = await gitFacts(store, task, opts.git ?? defaultGit);
  const last = task.runs[task.runs.length - 1];
  const lastResult = [...task.runs].reverse().find((r) => r.summary)?.summary;
  const status = [
    `- Estado de la tarea: **${task.state}**${last?.blockedReason ? ` (bloqueada por ${last.blockedReason})` : ''}`,
    `- Clasificación: ${task.classification} · modo: ${task.mode}`,
    task.branch
      ? `- Rama de trabajo: \`${task.branch}\` (base \`${task.baseBranch}\`)`
      : '- Sin rama de trabajo (tarea de solo lectura o sin ejecutar)',
  ];
  const raw = [
    `# Paquete de continuidad — ${task.title}`,
    '',
    `Tarea \`${task.id}\` · de \`${task.profileId}\` a \`${to.id}\``,
    '',
    section('Objetivo', [`Ver el brief original al final. Título: ${task.title}`]),
    section('Estado', status),
    section('Hechos verificados (observados por Hilvan)', [
      ...task.runs.map(runLine),
      ...facts.commits.map((c) => `- commit ${c}`),
    ]),
    section('Decisiones', [
      '- _(añada aquí decisiones tomadas que el siguiente agente deba respetar)_',
    ]),
    section('Supuestos y afirmaciones del agente anterior (NO verificados)', [
      lastResult
        ? `> ${lastResult.trim().split('\n').join('\n> ')}`
        : '_(el agente anterior no dejó resumen)_',
    ]),
    section(
      'Archivos y cambios',
      facts.files.map((f) => `- ${f.replace('\t', ' ')}`),
    ),
    section('Siguiente paso', [
      task.state === 'blocked'
        ? '- Continuar la tarea donde quedó; el perfil anterior alcanzó un límite.'
        : '- Continuar la tarea según el brief y verificar lo hecho en la rama.',
    ]),
    '---',
    '',
    '## Brief original',
    '',
    brief,
  ].join('\n');
  // SR-013: secrets never travel in a package; matches are redacted and reported.
  const { text, matches } = redactSecrets(raw);
  return {
    id: newHandoffId(),
    task,
    from: task.profileId,
    to,
    evaluation,
    body: text,
    secrets: matches,
  };
}

export function sha256(text: string): string {
  return createHash('sha256').update(text, 'utf8').digest('hex');
}

/**
 * Stores exactly the approved text (after any user edit), records its hash and moves the task
 * to the target profile. The caller must have shown the text and obtained approval.
 */
export async function approveHandoff(
  store: ProjectStore,
  draft: HandoffDraft,
  approvedBody: string,
): Promise<{ task: Task; hash: string; file: string }> {
  if (!draft.evaluation.eligible) {
    throw new HandoffError(
      `profile ${draft.to.id} is not eligible: ${draft.evaluation.exclusions.map((e) => e.reason).join('; ')}`,
    );
  }
  const leftover = findSecrets(approvedBody);
  if (leftover.length) {
    throw new HandoffError(
      `the edited package contains possible secrets (${leftover.map((s) => s.kind).join(', ')})`,
    );
  }
  const hash = sha256(approvedBody);
  const header = [
    '---',
    `id: ${draft.id}`,
    `task: ${draft.task.id}`,
    `from: ${draft.from}`,
    `to: ${draft.to.id}`,
    `classification: ${draft.task.classification}`,
    `sha256: ${hash}`,
    `approvedAt: ${new Date().toISOString()}`,
    `secretsRedacted: ${draft.secrets.length}`,
    '---',
    '',
  ].join('\n');
  const file = store.handoffFile(draft.task.id, draft.id);
  await mkdir(path.dirname(file), { recursive: true, mode: 0o700 });
  await writeFileAtomic(file, header + approvedBody);
  const current = await store.loadTask(draft.task.id);
  const task: Task = {
    ...current,
    profileId: draft.to.id,
    state: current.state === 'in_review' ? 'in_review' : 'ready',
    handoffs: [...current.handoffs, draft.id],
    approvedProfiles: [...new Set([...current.approvedProfiles, draft.to.id])],
  };
  await store.saveTask(task);
  await store.audit({
    type: 'handoff.approved',
    taskId: task.id,
    profileId: draft.to.id,
    data: {
      handoff: draft.id,
      from: draft.from,
      sha256: hash,
      secretsRedacted: draft.secrets.length,
    },
  });
  return { task, hash, file };
}

export async function cancelHandoff(store: ProjectStore, draft: HandoffDraft): Promise<void> {
  await store.audit({
    type: 'handoff.cancelled',
    taskId: draft.task.id,
    profileId: draft.to.id,
    data: { handoff: draft.id, from: draft.from },
  });
}
