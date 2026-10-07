import type { App, Profile } from '../domain/index.js';
import type { Command } from '../launchers/index.js';

/** Identity reported by the agent CLI for a profile; never contains credentials. */
export interface AuthStatus {
  loggedIn: boolean;
  /** e.g. `claude.ai`, `console`, `chatgpt`, `api-key`. */
  method?: string;
  /** Account identifier shown to the user (email or organisation), when the CLI exposes it. */
  account?: string;
  /** False when the CLI cannot tell which account is active (Codex). */
  verifiable: boolean;
  detail?: string;
}

/**
 * Per-application adapter (docs/design/contrato-adaptador.md).
 * M1 covers identity: environment isolation, login and auth checks. `start()` arrives in M2.
 */
export interface AgentAdapter {
  readonly app: App;
  /** CLI name used when the destination does not override it. */
  readonly defaultExecutable: string;
  /** Environment variable that isolates the profile (CLAUDE_CONFIG_DIR / CODEX_HOME). */
  readonly configDirEnv: string;
  /** Conventional directory of the CLI's default account, relative to the home directory. */
  readonly defaultConfigDir: string;
  /**
   * Environment for a profile: sets the config dir, maps an api-key profile's variable
   * to the provider's, and removes stray provider keys so a subscription profile
   * cannot silently authenticate as someone else (NFR-008).
   */
  profileEnv(profile: Profile, hostEnv: NodeJS.ProcessEnv): Pick<Command, 'env' | 'unsetEnv'>;
  /** Interactive login with the official flow (ADR-0006). */
  loginCommand(profile: Profile, hostEnv: NodeJS.ProcessEnv): Command;
  authStatusCommand(profile: Profile, hostEnv: NodeJS.ProcessEnv): Command;
  parseAuthStatus(stdout: string, stderr: string, exitCode: number | null): AuthStatus;
}
