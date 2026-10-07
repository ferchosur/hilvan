import path from 'node:path';
import { parse } from 'yaml';
import { z } from 'zod';
import { type EffectivePolicy, type PolicyFile, PolicyFileSchema } from '../domain/policy.js';
import { mergePolicies } from '../policy/evaluate.js';
import { readTextSafe, UnsafePathError } from '../security/fs.js';
import { managedPolicyFile, userPolicyFile } from './paths.js';
import { ConfigError, MAX_CONFIG_BYTES, YAML_OPTIONS } from './profiles.js';

export async function loadPolicyFile(file: string): Promise<PolicyFile | null> {
  let text: string | null;
  try {
    text = await readTextSafe(file, MAX_CONFIG_BYTES);
  } catch (err) {
    if (err instanceof UnsafePathError) throw new ConfigError(file, err.message);
    if ((err as NodeJS.ErrnoException).code === 'EACCES') return null;
    throw err;
  }
  if (text === null) return null;
  let raw: unknown;
  try {
    raw = parse(text, YAML_OPTIONS);
  } catch (err) {
    throw new ConfigError(file, (err as Error).message);
  }
  const r = PolicyFileSchema.safeParse(raw ?? { version: 1 });
  if (!r.success) throw new ConfigError(file, z.prettifyError(r.error));
  return r.data;
}

export interface PolicyPaths {
  managed: string;
  user: string;
  project?: string;
}

export function defaultPolicyPaths(projectRoot?: string): PolicyPaths {
  return {
    managed: managedPolicyFile(),
    user: userPolicyFile(),
    ...(projectRoot ? { project: path.join(projectRoot, '.hilvan', 'policy.yaml') } : {}),
  };
}

export async function loadEffectivePolicy(paths: PolicyPaths): Promise<EffectivePolicy> {
  const [managed, user, project] = await Promise.all([
    loadPolicyFile(paths.managed),
    loadPolicyFile(paths.user),
    paths.project ? loadPolicyFile(paths.project) : Promise.resolve(null),
  ]);
  return mergePolicies({ managed, user, project });
}
