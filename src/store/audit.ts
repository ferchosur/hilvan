import { createHash } from 'node:crypto';
import { appendFile, open } from 'node:fs/promises';
import path from 'node:path';
import { ensurePrivateDir, readTextSafe } from '../security/fs.js';

/**
 * SR-017/SR-018: append-only audit log as a hash chain.
 * Each line is `{ ...event, prev, hash }`, where `hash = sha256(prev + canonical(event))`.
 * Editing, deleting or reordering any line breaks verification.
 */

export const GENESIS = '0'.repeat(64);

export interface AuditEvent {
  ts: string;
  type: string;
  taskId?: string;
  runId?: string;
  profileId?: string;
  /** Hashes, ids and paths only — never package contents or secrets. */
  data?: Record<string, unknown>;
}

export interface ChainedEvent extends AuditEvent {
  prev: string;
  hash: string;
}

/** JSON with sorted keys so the hash does not depend on property order. */
export function canonical(value: unknown): string {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
  const entries = Object.entries(value as Record<string, unknown>)
    .filter(([, v]) => v !== undefined)
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
  return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${canonical(v)}`).join(',')}}`;
}

export function chainHash(prev: string, event: AuditEvent): string {
  return createHash('sha256').update(prev).update(canonical(event)).digest('hex');
}

/** Reads the hash of the last line without loading the whole file. */
async function lastHash(file: string): Promise<string> {
  let fh: Awaited<ReturnType<typeof open>>;
  try {
    fh = await open(file, 'r');
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') return GENESIS;
    throw err;
  }
  try {
    const { size } = await fh.stat();
    if (size === 0) return GENESIS;
    const len = Math.min(size, 64 * 1024);
    const buf = Buffer.alloc(len);
    await fh.read(buf, 0, len, size - len);
    const lines = buf.toString('utf8').trimEnd().split('\n');
    const last = lines[lines.length - 1] ?? '';
    const parsed = JSON.parse(last) as Partial<ChainedEvent>;
    if (typeof parsed.hash !== 'string') throw new Error('audit log tail is malformed');
    return parsed.hash;
  } finally {
    await fh.close();
  }
}

/** Appends an event to the chain. Single writer per project (guarded by the project lock in M2). */
export async function appendAudit(file: string, event: AuditEvent): Promise<ChainedEvent> {
  await ensurePrivateDir(path.dirname(file));
  const prev = await lastHash(file);
  const chained: ChainedEvent = { ...event, prev, hash: chainHash(prev, event) };
  await appendFile(file, `${JSON.stringify(chained)}\n`, { mode: 0o600 });
  return chained;
}

export interface VerifyResult {
  ok: boolean;
  events: number;
  /** 1-based line of the first problem. */
  brokenAt?: number;
  reason?: string;
}

export async function verifyAudit(
  file: string,
  maxBytes = 512 * 1024 * 1024,
): Promise<VerifyResult> {
  const text = await readTextSafe(file, maxBytes);
  if (text === null) return { ok: true, events: 0 };
  let prev = GENESIS;
  const lines = text.split('\n').filter((l) => l.length > 0);
  for (let i = 0; i < lines.length; i++) {
    let ev: ChainedEvent;
    try {
      ev = JSON.parse(lines[i] ?? '') as ChainedEvent;
    } catch {
      return { ok: false, events: i, brokenAt: i + 1, reason: 'invalid JSON' };
    }
    const { prev: p, hash, ...event } = ev;
    if (p !== prev) return { ok: false, events: i, brokenAt: i + 1, reason: 'chain link mismatch' };
    if (chainHash(prev, event) !== hash) {
      return { ok: false, events: i, brokenAt: i + 1, reason: 'content was modified' };
    }
    prev = hash;
  }
  return { ok: true, events: lines.length };
}
