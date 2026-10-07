import { z } from 'zod';

/** Stable identifier used for profiles, tags and rule ids: lowercase, digits and dashes. */
export const IdSchema = z
  .string()
  .regex(/^[a-z0-9][a-z0-9-]{0,62}$/, 'must be lowercase letters, digits or dashes (max 63)');

/** Agent applications with an adapter (D-015). New apps are added here and in `adapters/`. */
export const AppSchema = z.enum(['claude-code', 'codex']);
export type App = z.infer<typeof AppSchema>;

/** How the command reaches its execution environment (ADR-0005). `ssh` arrives in increment 2. */
export const LauncherSchema = z.enum(['native', 'wsl', 'windows']);
export type Launcher = z.infer<typeof LauncherSchema>;

export const DestinationSchema = z
  .object({
    launcher: LauncherSchema,
    /** WSL distribution name, required when launcher is `wsl`. */
    distro: z.string().min(1).optional(),
    /** Executable override; defaults to the app's CLI name. */
    executable: z.string().min(1).optional(),
  })
  .superRefine((d, ctx) => {
    if (d.launcher === 'wsl' && !d.distro) {
      ctx.addIssue({ code: 'custom', path: ['distro'], message: 'required when launcher is wsl' });
    }
  });
export type Destination = z.infer<typeof DestinationSchema>;

/**
 * Authentication mode (ADR-0006). The orchestrator never stores credentials:
 * `subscription` relies on the official CLI login inside `configDir`;
 * `api-key` references an environment variable by name.
 */
export const AuthSchema = z.discriminatedUnion('mode', [
  z.object({ mode: z.literal('subscription') }),
  z.object({
    mode: z.literal('api-key'),
    apiKeyEnv: z.string().regex(/^[A-Z_][A-Z0-9_]*$/, 'must be an environment variable name'),
  }),
]);
export type Auth = z.infer<typeof AuthSchema>;

export const ProfileSchema = z.object({
  id: IdSchema,
  label: z.string().min(1),
  app: AppSchema,
  auth: AuthSchema,
  /**
   * CLAUDE_CONFIG_DIR / CODEX_HOME, as a path inside the destination environment.
   * `null` means the CLI's default login: the variable is removed, not set, because
   * Claude Code keeps the default account outside `~/.claude` (in `~/.claude.json`).
   */
  configDir: z.string().min(1).nullable().default(null),
  destination: DestinationSchema,
  /** Data classifications this profile may receive (D-022). */
  accepts: z.array(z.string().min(1)).min(1),
  tags: z.array(IdSchema).default([]),
  tools: z
    .object({
      /** MCP configuration file passed with --strict-mcp-config (NFR-011). */
      mcpConfig: z.string().min(1).optional(),
      /** Capabilities declared for eligibility checks (FR-006, FR-011). */
      provides: z.array(IdSchema).default([]),
    })
    .default({ provides: [] }),
  model: z.string().min(1).nullable().default(null),
  /** Tie-breaker among eligible profiles; higher is preferred. */
  preference: z.number().int().default(0),
});
export type Profile = z.infer<typeof ProfileSchema>;

export const ProfilesFileSchema = z
  .object({
    version: z.literal(1),
    profiles: z.array(ProfileSchema).default([]),
  })
  .superRefine((file, ctx) => {
    const seen = new Set<string>();
    file.profiles.forEach((p, i) => {
      if (seen.has(p.id)) {
        ctx.addIssue({
          code: 'custom',
          path: ['profiles', i, 'id'],
          message: `duplicate id "${p.id}"`,
        });
      }
      seen.add(p.id);
    });
  });
export type ProfilesFile = z.infer<typeof ProfilesFileSchema>;
