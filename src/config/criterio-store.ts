import path from 'node:path';
import { z } from 'zod';
import { Criterio, type CriterioState, TASK_TYPES } from '../criterio/index.js';
import { readTextSafe, UnsafePathError, writeFileAtomic } from '../security/fs.js';

const SoftmaxStateSchema = z.object({
  classes: z.number().int().min(2).max(64),
  weights: z.record(z.string().regex(/^\d+$/), z.array(z.number().finite())),
  updates: z.number().int().nonnegative(),
});

const ArmSchema = z.object({ a: z.number().positive(), b: z.number().positive() });

/** Validates a stored model so a corrupted or tampered file cannot crash or skew Hilvan. */
export const CriterioStateSchema = z.object({
  version: z.literal(1),
  taskType: SoftmaxStateSchema.refine((s) => s.classes === TASK_TYPES.length),
  sensitivity: SoftmaxStateSchema.refine((s) => s.classes === 2),
  acceptance: SoftmaxStateSchema.refine((s) => s.classes === 2),
  tools: z
    .record(
      z.string().regex(/^[a-z0-9][a-z0-9-]{0,62}$/),
      SoftmaxStateSchema.refine((s) => s.classes === 2),
    )
    .optional()
    .refine((t) => !t || Object.keys(t).length <= 200, 'too many tool models'),
  complexityOffset: z.partialRecord(
    z.enum(TASK_TYPES),
    z.object({ sum: z.number().finite(), n: z.number().int().nonnegative() }),
  ),
  arms: z.record(z.string().max(200), ArmSchema),
  history: z
    .array(
      z.object({
        q: z.enum(['taskType', 'sensitivity', 'acceptance', 'tools']),
        p: z.number().min(0).max(1),
        y: z.union([z.literal(0), z.literal(1)]),
      }),
    )
    .max(1000),
});

export const MAX_MODEL_BYTES = 8 * 1024 * 1024;

export function criterioFile(configDir: string): string {
  return path.join(configDir, 'criterio', 'model.json');
}

/** Loads the learned model; a missing or invalid file yields a fresh engine (rules only). */
export async function loadCriterio(file: string): Promise<{ engine: Criterio; warning?: string }> {
  let text: string | null;
  try {
    text = await readTextSafe(file, MAX_MODEL_BYTES);
  } catch (err) {
    if (err instanceof UnsafePathError) {
      const reason = /exceeds/.test(err.message) ? 'is too large' : 'is not a regular file';
      return { engine: new Criterio(), warning: `${file} ${reason}; ignored` };
    }
    throw err;
  }
  if (text === null) return { engine: new Criterio() };
  try {
    const state = CriterioStateSchema.parse(JSON.parse(text)) as CriterioState;
    return { engine: new Criterio(state) };
  } catch {
    return { engine: new Criterio(), warning: `${file} is invalid; using rules only` };
  }
}

export async function saveCriterio(file: string, engine: Criterio): Promise<void> {
  await writeFileAtomic(file, JSON.stringify(engine.toJSON()));
}
