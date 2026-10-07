import { z } from 'zod';
import { IdSchema } from './profile.js';

/** Layered policy (ADR-0009, docs/design/modelo-de-datos.md §2). */

export const DEFAULT_CLASSIFICATIONS = ['public', 'personal', 'corporate', 'confidential'] as const;

const Selector = z
  .object({
    classification: z.array(z.string().min(1)).optional(),
    profiles: z.array(IdSchema).optional(),
    tags: z.array(IdSchema).optional(),
    apps: z.array(z.string().min(1)).optional(),
  })
  .strict();

export const RuleSchema = z
  .object({
    id: IdSchema,
    /** Data being sent: task classification and/or the profile it comes from (hand-offs). */
    from: Selector.default({}),
    /** Profile receiving the data. */
    to: Selector.default({}),
    effect: z.enum(['deny', 'require_approval']),
    /** Only honoured in the managed policy (SR-020); ignored elsewhere. */
    enforced: z.boolean().default(false),
    reason: z.string().min(1).max(500),
  })
  .strict();
export type Rule = z.infer<typeof RuleSchema>;

export const PolicyFileSchema = z
  .object({
    version: z.literal(1),
    classifications: z.array(z.string().min(1)).min(1).optional(),
    rules: z.array(RuleSchema).default([]),
  })
  .strict();
export type PolicyFile = z.infer<typeof PolicyFileSchema>;

export type PolicyLayer = 'managed' | 'user' | 'project';

export interface LayeredRule extends Rule {
  layer: PolicyLayer;
}

export interface EffectivePolicy {
  classifications: string[];
  rules: LayeredRule[];
  warnings: string[];
}
