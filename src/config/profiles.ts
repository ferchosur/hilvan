import { parse, stringify } from 'yaml';
import { z } from 'zod';
import { type ProfilesFile, ProfilesFileSchema } from '../domain/index.js';
import { readTextSafe, UnsafePathError, writeFileAtomic } from '../security/fs.js';

/** SR-008: YAML limits. */
export const MAX_CONFIG_BYTES = 1024 * 1024;
export const YAML_OPTIONS = { maxAliasCount: 100, uniqueKeys: true } as const;

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
    if (text.length > MAX_CONFIG_BYTES) throw new Error(`file exceeds ${MAX_CONFIG_BYTES} bytes`);
    raw = parse(text, YAML_OPTIONS);
  } catch (err) {
    throw new ConfigError(file, err instanceof Error ? err.message : String(err));
  }
  const result = ProfilesFileSchema.safeParse(raw ?? { version: 1 });
  if (!result.success) throw new ConfigError(file, z.prettifyError(result.error));
  return result.data;
}

/** Loads `profiles.yaml`; a missing file means no profiles configured yet. */
export async function loadProfiles(file: string): Promise<ProfilesFile> {
  let text: string | null;
  try {
    text = await readTextSafe(file, MAX_CONFIG_BYTES);
  } catch (err) {
    if (err instanceof UnsafePathError) throw new ConfigError(file, err.message);
    throw err;
  }
  if (text === null) return { version: 1, profiles: [] };
  return parseProfiles(text, file);
}

/** Writes `profiles.yaml` atomically; the config directory is private to the user. */
export async function saveProfiles(file: string, data: ProfilesFile): Promise<void> {
  const valid = ProfilesFileSchema.parse(data);
  const header = '# Hilvan profiles — managed with `hilvan profile`. Never put secrets here.\n';
  await writeFileAtomic(file, header + stringify(valid));
}
