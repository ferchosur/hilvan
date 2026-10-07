import type { Profile } from '../domain/index.js';
import { parseClaudeLine } from './parsers.js';
import type { AgentAdapter, AuthStatus } from './types.js';

const PROVIDER_KEYS = ['ANTHROPIC_API_KEY', 'ANTHROPIC_AUTH_TOKEN'] as const;

function profileEnv(profile: Profile, hostEnv: NodeJS.ProcessEnv) {
  const env: Record<string, string> = {};
  const unsetDir: string[] = [];
  if (profile.configDir) env.CLAUDE_CONFIG_DIR = profile.configDir;
  else unsetDir.push('CLAUDE_CONFIG_DIR');
  if (profile.auth.mode === 'api-key') {
    const value = hostEnv[profile.auth.apiKeyEnv];
    if (value) env.ANTHROPIC_API_KEY = value;
    return { env, unsetEnv: [...unsetDir, 'ANTHROPIC_AUTH_TOKEN'] };
  }
  return { env, unsetEnv: [...unsetDir, ...PROVIDER_KEYS] };
}

export const claudeCodeAdapter: AgentAdapter = {
  app: 'claude-code',
  defaultExecutable: 'claude',
  configDirEnv: 'CLAUDE_CONFIG_DIR',
  defaultConfigDir: '.claude',
  profileEnv,

  loginCommand(profile, hostEnv) {
    return { executable: 'claude', args: ['auth', 'login'], ...profileEnv(profile, hostEnv) };
  },

  authStatusCommand(profile, hostEnv) {
    return {
      executable: 'claude',
      args: ['auth', 'status', '--json'],
      ...profileEnv(profile, hostEnv),
    };
  },

  /** docs/design/contrato-adaptador.md §4; NFR-011/SR-011: only the profile's MCP servers. */
  runCommand(profile, req, hostEnv) {
    const args = [
      '-p',
      '--output-format',
      'stream-json',
      '--verbose',
      '--strict-mcp-config',
      ...(profile.tools.mcpConfig ? ['--mcp-config', profile.tools.mcpConfig] : []),
      '--permission-prompts',
      'none',
      '--permission-mode',
      req.mode === 'write' ? 'acceptEdits' : 'dontAsk',
      ...((req.model ?? profile.model) ? ['--model', (req.model ?? profile.model) as string] : []),
      ...(req.resumeSessionId ? ['--resume', req.resumeSessionId] : []),
    ];
    return { executable: 'claude', args, cwd: req.cwd, ...profileEnv(profile, hostEnv) };
  },

  createParser() {
    return parseClaudeLine;
  },

  parseAuthStatus(stdout, stderr, exitCode): AuthStatus {
    let data: Record<string, unknown>;
    try {
      data = JSON.parse(stdout) as Record<string, unknown>;
    } catch {
      const detail = (stderr || stdout).trim().split(/\r?\n/)[0] ?? `exit ${exitCode}`;
      return { loggedIn: false, verifiable: true, detail: `unreadable auth status: ${detail}` };
    }
    const loggedIn = data.loggedIn === true;
    const method = typeof data.authMethod === 'string' ? data.authMethod : undefined;
    const email = typeof data.email === 'string' ? data.email : undefined;
    const org = typeof data.orgName === 'string' ? data.orgName : undefined;
    const account = email ?? org;
    return {
      loggedIn,
      verifiable: true,
      ...(method ? { method } : {}),
      ...(account ? { account } : {}),
      ...(org && email && !org.includes(email) ? { detail: `org: ${org}` } : {}),
    };
  },
};
