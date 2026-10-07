import type { Profile } from '../domain/index.js';
import { parseClaudeLine } from './parsers.js';
import type { AgentAdapter, AuthStatus, DiscoveredTool } from './types.js';

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
      // ADR-0013: never load repository settings, hooks or .mcp.json (verified empirically:
      // --strict-mcp-config alone still runs repository hooks).
      '--setting-sources',
      'user',
      ...(profile.tools.mcp === 'account' ? [] : ['--strict-mcp-config']),
      ...(profile.tools.mcp === 'profile' && profile.tools.mcpConfig
        ? ['--mcp-config', profile.tools.mcpConfig]
        : []),
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

  listToolsCommand(profile, hostEnv) {
    return { executable: 'claude', args: ['mcp', 'list'], ...profileEnv(profile, hostEnv) };
  },

  /** Lines like `name: <target> - ✔ Connected`; the target is not kept (it may embed tokens). */
  parseToolList(stdout) {
    const out: DiscoveredTool[] = [];
    for (const line of stdout.split(/\r?\n/)) {
      const m = /^(.+?):\s.*\s-\s(\S+)\s*(.*)$/.exec(line.trim());
      if (!m?.[1] || /checking mcp/i.test(line)) continue;
      const st = `${m[2]} ${m[3]}`.toLowerCase();
      const status = /connected|✔|✓/.test(st)
        ? 'connected'
        : /fail|✗|error/.test(st)
          ? 'failed'
          : /pending|⏸/.test(st)
            ? 'pending'
            : 'unknown';
      out.push({ name: m[1].trim(), status });
    }
    return out;
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
