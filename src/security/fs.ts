import { randomBytes } from 'node:crypto';
import { constants } from 'node:fs';
import { lstat, mkdir, open, rename, rm } from 'node:fs/promises';
import path from 'node:path';

/**
 * SR-006/SR-007: file access that refuses symlinks and non-regular files,
 * bounds sizes and writes atomically with private permissions.
 */

export class UnsafePathError extends Error {
  override name = 'UnsafePathError';
}

const NOFOLLOW = constants.O_NOFOLLOW ?? 0;

/**
 * Reads a regular file as UTF-8. Returns null when it does not exist.
 * Rejects symlinks (O_NOFOLLOW where available, lstat elsewhere) and files above `maxBytes`.
 */
export async function readTextSafe(file: string, maxBytes: number): Promise<string | null> {
  if (!NOFOLLOW) {
    try {
      if ((await lstat(file)).isSymbolicLink()) throw new UnsafePathError(`${file} is a symlink`);
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code === 'ENOENT') return null;
      throw err;
    }
  }
  let fh: Awaited<ReturnType<typeof open>>;
  try {
    fh = await open(file, constants.O_RDONLY | NOFOLLOW);
  } catch (err) {
    const code = (err as NodeJS.ErrnoException).code;
    if (code === 'ENOENT') return null;
    if (code === 'ELOOP') throw new UnsafePathError(`${file} is a symlink`);
    throw err;
  }
  try {
    const st = await fh.stat();
    if (!st.isFile()) throw new UnsafePathError(`${file} is not a regular file`);
    if (st.size > maxBytes) throw new UnsafePathError(`${file} exceeds ${maxBytes} bytes`);
    const buf = Buffer.alloc(st.size);
    let read = 0;
    while (read < st.size) {
      const { bytesRead } = await fh.read(buf, read, st.size - read, read);
      if (bytesRead === 0) break;
      read += bytesRead;
    }
    return buf.subarray(0, read).toString('utf8');
  } finally {
    await fh.close();
  }
}

/** Creates a directory tree with 0700 for the directories it creates. */
export async function ensurePrivateDir(dir: string): Promise<void> {
  await mkdir(dir, { recursive: true, mode: 0o700 });
}

/**
 * Atomic write: random temp file created exclusively (O_EXCL, 0600) next to the target,
 * fsync, then rename. Refuses to replace a symlink.
 */
export async function writeFileAtomic(file: string, data: string, mode = 0o600): Promise<void> {
  const dir = path.dirname(file);
  await ensurePrivateDir(dir);
  try {
    if ((await lstat(file)).isSymbolicLink()) throw new UnsafePathError(`${file} is a symlink`);
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code !== 'ENOENT') throw err;
  }
  const tmp = path.join(dir, `.${path.basename(file)}.${randomBytes(6).toString('hex')}.tmp`);
  const fh = await open(tmp, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL, mode);
  try {
    await fh.writeFile(data, 'utf8');
    await fh.sync();
  } catch (err) {
    await fh.close();
    await rm(tmp, { force: true });
    throw err;
  }
  await fh.close();
  try {
    await rename(tmp, file);
  } catch (err) {
    await rm(tmp, { force: true });
    throw err;
  }
}

/** Unix permission problems for a config file: writable by group/others or not owned by us. */
export async function looseConfigPermissions(file: string): Promise<string | null> {
  if (process.platform === 'win32') return null;
  try {
    const st = await lstat(file);
    if (st.isSymbolicLink()) return `${file} is a symlink`;
    const uid = process.getuid?.();
    if (uid !== undefined && st.uid !== uid) return `${file} is owned by uid ${st.uid}`;
    if (st.mode & 0o022)
      return `${file} is writable by group/others (${(st.mode & 0o777).toString(8)})`;
    return null;
  } catch {
    return null;
  }
}
