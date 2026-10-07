import type { Profile } from '../domain/index.js';
import { type CodexParserState, parseCodexLine } from './parsers.js';
import type { AgentAdapter, AuthStatus } from './types.js';

const PROVIDER_KEYS = ['OPENAI_API_KEY', 'CODEX_API_KEY'] as const;

function profileEnv(profile: Profile, hostEnv: NodeJS.ProcessEnv) {
  const env: Record<string, string> = {};
  const unsetDir: string[] = [];
  if (profile.configDir) env.CODEX_HOME = profile.configDir;
  else unsetDir.push('CODEX_HOME');
  if (profile.auth.mode === 'api-key') {
    const value = hostEnv[profile.auth.apiKeyEnv];
    if (value) env.CODEX_API_KEY = value;
    return { env, unsetEnv: [...unsetDir, 'OPENAI_API_KEY'] };
  }
  return { env, unsetEnv: [...unsetDir, ...PROVIDER_KEYS] };
}

export const codexAdapter: AgentAdapter = {
  app: 'codex',
  defaultExecutable: 'codex',
  configDirEnv: 'CODEX_HOME',
  defaultConfigDir: '.codex',
  profileEnv,

  loginCommand(profile, hostEnv) {
    return { executable: 'codex', args: ['login'], ...profileEnv(profile, hostEnv) };
  },

  authStatusCommand(profile, hostEnv) {
    return { executable: 'codex', args: ['login', 'status'], ...profileEnv(profile, hostEnv) };
  },

  runCommand(profile, req, hostEnv) {
    const model = req.model ?? profile.model;
    const common = [
      '--json',
      '--cd',
      req.cwd,
      '--sandbox',
      req.mode === 'write' ? 'workspace-write' : 'read-only',
      ...(model ? ['--model', model] : []),
    ];
    const args = req.resumeSessionId
      ? ['exec', 'resume', req.resumeSessionId, ...common, '-']
      : ['exec', ...common, '-'];
    return { executable: 'codex', args, cwd: req.cwd, ...profileEnv(profile, hostEnv) };
  },

  createParser() {
    const st: CodexParserState = { lastMessage: '' };
    return (line: string) => parseCodexLine(line, st);
  },

  /** `codex login status` prints text and does not identify the account (spike 002). */
  parseAuthStatus(stdout, stderr, exitCode): AuthStatus {
    const text = `${stdout}\n${stderr}`;
    const line = text
      .split(/\r?\n/)
      .find((l) => /logged in/i.test(l))
      ?.trim();
    if (line && !/not logged in/i.test(line)) {
      const method = /chatgpt/i.test(line)
        ? 'chatgpt'
        : /api key/i.test(line)
          ? 'api-key'
          : undefined;
      return {
        loggedIn: true,
        verifiable: false,
        ...(method ? { method } : {}),
        detail: 'Codex does not report which account is active',
      };
    }
    const err = text
      .split(/\r?\n/)
      .find((l) => /error/i.test(l))
      ?.trim();
    return {
      loggedIn: false,
      verifiable: false,
      detail: line ?? err ?? `exit ${exitCode}`,
    };
  },
};
