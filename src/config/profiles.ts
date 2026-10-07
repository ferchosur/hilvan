import { mkdir, readFile, rename, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { parse, stringify } from 'yaml';
import { z } from 'zod';
import { type ProfilesFile, ProfilesFileSchema } from '../domain/index.js';

export class ConfigError extends Error {
  constructor(
    readonly file: string,
    readonly details: string,
  ) {
    super(`Invalid configuration in ${file}:\n${details}`);
    this.name = 'ConfigError';
  }
}

/** Parses and validates the text of a `profiles.yaml` file. */
export function parseProfiles(text: string, file = 'profiles.yaml'): ProfilesFile {
  let raw: unknown;
  try {
    raw = parse(text);
  } catch (err) {
    throw new ConfigError(file, err instanceof Error ? err.message : String(err));
  }
  const result = ProfilesFileSchema.safeParse(raw ?? { version: 1 });
  if (!result.success) throw new ConfigError(file, z.prettifyError(result.error));
  return result.data;
}

/** Loads `profiles.yaml`; a missing file means no profiles configured yet. */
export async function loadProfiles(file: string): Promise<ProfilesFile> {
  let text: string;
  try {
    text = await readFile(file, 'utf8');
  } catch (err) {
    if ((err as NodeJS.ErrnoException).code === 'ENOENT') return { version: 1, profiles: [] };
    throw err;
  }
  return parseProfiles(text, file);
}

/** Writes `profiles.yaml` atomically; the config directory is private to the user. */
export async function saveProfiles(file: string, data: ProfilesFile): Promise<void> {
  const valid = ProfilesFileSchema.parse(data);
  await mkdir(path.dirname(file), { recursive: true, mode: 0o700 });
  const tmp = `${file}.${process.pid}.tmp`;
  const header = '# Hilvan profiles — managed with `hilvan profile`. Never put secrets here.\n';
  await writeFile(tmp, header + stringify(valid), { mode: 0o600 });
  await rename(tmp, file);
}
