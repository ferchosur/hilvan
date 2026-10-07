import { chmod, mkdtemp, readFile, rm, stat, symlink, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { ConfigError, loadProfiles, parseProfiles } from '../src/config/profiles.js';
import { LineLimitError, LineSplitter } from '../src/process/lines.js';
import { runCommand } from '../src/process/run.js';
import { assertSafeArgs, UnsafeArgumentError } from '../src/security/args.js';
import {
  looseConfigPermissions,
  readTextSafe,
  UnsafePathError,
  writeFileAtomic,
} from '../src/security/fs.js';
import { hasControlSequences, sanitizeForTerminal } from '../src/security/sanitize.js';
import { entropy, findSecrets, redactSecrets } from '../src/security/secrets.js';
import { appendAudit, verifyAudit } from '../src/store/audit.js';

const posix = process.platform !== 'win32';

describe('SR-001 terminal sanitising', () => {
  it.each([
    ['CSI colour', 'ok \u001b[31mred\u001b[0m', 'ok red'],
    ['cursor movement', 'a\u001b[2Kb\u001b[1A', 'ab'],
    ['OSC 52 clipboard write', 'x\u001b]52;c;ZXZpbA==\u0007y', 'xy'],
    ['OSC 8 hyperlink', '\u001b]8;;http://evil\u001b\\click\u001b]8;;\u001b\\', 'click'],
    ['DCS', 'a\u001bPq#0;2;0;0;0\u001b\\b', 'ab'],
    ['8-bit CSI', 'a\u009b31mb', 'ab'],
    ['bell and backspace', 'a\u0007\u0008b', 'ab'],
    ['bidi override (Trojan Source)', 'admin‮⁦ // user', 'admin // user'],
    ['zero width', 'pa​ss', 'pass'],
  ])('%s', (_name, input, expected) => {
    expect(sanitizeForTerminal(input)).toBe(expected);
    expect(hasControlSequences(input)).toBe(true);
  });

  it('keeps newlines and tabs unless single-line is requested', () => {
    expect(sanitizeForTerminal('a\n\tb\r\nc')).toBe('a\n\tb\nc');
    expect(sanitizeForTerminal('a\nb', { multiline: false })).toBe('a b');
    expect(sanitizeForTerminal('abcdef', { maxLength: 4 })).toBe('abc…');
    expect(hasControlSequences('plain text\nok')).toBe(false);
  });
});

describe('SR-013 secret detection', () => {
  it.each([
    ['anthropic-key', `key sk-ant-api03-${'a1B2'.repeat(10)}`],
    ['openai-key', `OPENAI=sk-proj-${'Zx9'.repeat(12)}`],
    ['github-token', `ghp_${'A1b2C3d4'.repeat(5)}`],
    ['aws-access-key', 'AKIAIOSFODNN7EXAMPLE'],
    ['private-key', '-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----'],
    ['url-credentials', 'postgres://admin:hunter2pw@db.internal:5432/app'],
    ['assignment', 'password = "correcthorse99"'],
    ['jwt', 'eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abcdefghijklmnopqrstu'],
  ])('detects %s', (kind, text) => {
    expect(findSecrets(text).map((m) => m.kind)).toContain(kind);
  });

  it('flags high-entropy tokens but not words, paths or hex hashes', () => {
    expect(findSecrets('token Xk9mQ2vL7pR4sT8wZ1yB6nC3dF5gH0jK').map((m) => m.kind)).toEqual([
      'high-entropy',
    ]);
    expect(findSecrets('src/components/navigation/TopBarMenuItem.tsx')).toEqual([]);
    expect(findSecrets('commit 4e6559850ad2c3b1a9f0e8d7c6b5a4f3e2d1c0b9')).toEqual([]);
    expect(entropy('aaaa')).toBe(0);
  });

  it('redacts in place', () => {
    const { text, matches } = redactSecrets('use AKIAIOSFODNN7EXAMPLE now');
    expect(text).toBe('use [REDACTED:aws-access-key] now');
    expect(matches).toHaveLength(1);
  });
});

describe('SR-004 argument safety', () => {
  it('rejects newlines and NUL in arguments', async () => {
    expect(() => assertSafeArgs(['ok', 'a\nb'])).toThrow(UnsafeArgumentError);
    expect(() => assertSafeArgs(['a\u0000'])).toThrow(UnsafeArgumentError);
    expect(() => assertSafeArgs(['--flag=value with spaces'])).not.toThrow();
    await expect(runCommand('echo', ['x\ny'])).rejects.toThrow(UnsafeArgumentError);
  });
});

describe('SR-009 line limits', () => {
  it('splits chunks across boundaries and drops blank lines', () => {
    const lines: string[] = [];
    const s = new LineSplitter((l) => lines.push(l));
    s.push('{"a":1}\n{"b"');
    s.push(':2}\r\n\n{"c":3}');
    s.end();
    expect(lines).toEqual(['{"a":1}', '{"b":2}', '{"c":3}']);
  });

  it('enforces per-line and total limits', () => {
    const s = new LineSplitter(() => {}, { maxLine: 10 });
    expect(() => s.push('x'.repeat(11))).toThrow(LineLimitError);
    const t = new LineSplitter(() => {}, { maxTotal: 20 });
    expect(() => t.push(`${'y\n'.repeat(11)}`)).toThrow(LineLimitError);
  });
});

describe('filesystem safety', () => {
  let dir: string;
  beforeEach(async () => {
    dir = await mkdtemp(path.join(os.tmpdir(), 'hilvan-sec-'));
  });
  afterEach(async () => {
    await rm(dir, { recursive: true, force: true });
  });

  it('SR-007 writes atomically with 0600 and leaves no temp files', async () => {
    const f = path.join(dir, 'sub', 'a.yaml');
    await writeFileAtomic(f, 'one');
    await writeFileAtomic(f, 'two');
    expect(await readFile(f, 'utf8')).toBe('two');
    if (posix) {
      expect((await stat(f)).mode & 0o777).toBe(0o600);
      expect((await stat(path.dirname(f))).mode & 0o777).toBe(0o700);
    }
    const { readdir } = await import('node:fs/promises');
    expect(await readdir(path.dirname(f))).toEqual(['a.yaml']);
  });

  it.runIf(posix)('SR-006/007 refuses to read or replace symlinks', async () => {
    const target = path.join(dir, 'victim.txt');
    await writeFile(target, 'precious');
    const link = path.join(dir, 'profiles.yaml');
    await symlink(target, link);
    await expect(readTextSafe(link, 1000)).rejects.toThrow(UnsafePathError);
    await expect(writeFileAtomic(link, 'pwned')).rejects.toThrow(UnsafePathError);
    expect(await readFile(target, 'utf8')).toBe('precious');
    await expect(loadProfiles(link)).rejects.toThrow(ConfigError);
  });

  it('SR-006 enforces size limits and returns null when missing', async () => {
    const f = path.join(dir, 'big');
    await writeFile(f, 'x'.repeat(100));
    await expect(readTextSafe(f, 10)).rejects.toThrow(UnsafePathError);
    expect(await readTextSafe(path.join(dir, 'nope'), 10)).toBeNull();
  });

  it.runIf(posix)('SR-005 reports group/other-writable config files', async () => {
    const f = path.join(dir, 'profiles.yaml');
    await writeFile(f, 'version: 1');
    await chmod(f, 0o666);
    expect(await looseConfigPermissions(f)).toMatch(/writable by group\/others/);
    await chmod(f, 0o600);
    expect(await looseConfigPermissions(f)).toBeNull();
  });
});

describe('SR-008 YAML limits', () => {
  it('rejects alias bombs and duplicate keys', () => {
    const levels = ['a: &a ["x","x","x","x","x","x","x","x","x","x"]'];
    for (let i = 1; i < 12; i++) {
      const prev = String.fromCharCode(96 + i);
      const cur = String.fromCharCode(97 + i);
      levels.push(`${cur}: &${cur} [${Array(10).fill(`*${prev}`).join(',')}]`);
    }
    expect(() => parseProfiles(levels.join('\n'))).toThrow(ConfigError);
    expect(() => parseProfiles('version: 1\nversion: 1')).toThrow(ConfigError);
  });
});

describe('SR-017 audit hash chain', () => {
  let dir: string;
  beforeEach(async () => {
    dir = await mkdtemp(path.join(os.tmpdir(), 'hilvan-audit-'));
  });
  afterEach(async () => {
    await rm(dir, { recursive: true, force: true });
  });

  const ev = (i: number) => ({
    ts: `2026-10-07T00:00:0${i}Z`,
    type: 'run.started',
    taskId: `t${i}`,
    data: { sha256: 'ab'.repeat(32), n: i },
  });

  it('verifies an intact chain and detects edits, deletions and reordering', async () => {
    const f = path.join(dir, '.hilvan', 'events.jsonl');
    for (let i = 0; i < 4; i++) await appendAudit(f, ev(i));
    expect(await verifyAudit(f)).toEqual({ ok: true, events: 4 });

    const lines = (await readFile(f, 'utf8')).trimEnd().split('\n');
    const write = (ls: string[]) => writeFile(f, `${ls.join('\n')}\n`);

    await write([lines[0] ?? '', (lines[1] ?? '').replace('"t1"', '"tX"'), ...lines.slice(2)]);
    expect(await verifyAudit(f)).toMatchObject({
      ok: false,
      brokenAt: 2,
      reason: 'content was modified',
    });

    await write([lines[0] ?? '', ...lines.slice(2)]);
    expect(await verifyAudit(f)).toMatchObject({
      ok: false,
      brokenAt: 2,
      reason: 'chain link mismatch',
    });

    await write([lines[1] ?? '', lines[0] ?? '', ...lines.slice(2)]);
    expect((await verifyAudit(f)).ok).toBe(false);

    await write(lines);
    expect((await verifyAudit(f)).ok).toBe(true);
  });

  it('treats a missing log as empty', async () => {
    expect(await verifyAudit(path.join(dir, 'none.jsonl'))).toEqual({ ok: true, events: 0 });
  });
});
