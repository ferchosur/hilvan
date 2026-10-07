import * as p from '@clack/prompts';
import type { Command } from 'commander';
import { stringify } from 'yaml';
import { z } from 'zod';
import { adapterFor } from '../adapters/index.js';
import { profilesFile } from '../config/paths.js';
import { loadProfiles, saveProfiles } from '../config/profiles.js';
import {
  addProfile,
  type CheckDeps,
  checkProfile,
  configDirIssues,
  currentHost,
  defaultConfigDir,
  describeDestination,
  discoverTools,
  ensureConfigDir,
  expandHome,
  findProfile,
  type ProfileCheck,
  removeProfile,
  sharedAccountWarnings,
} from '../core/profiles.js';
import { AppSchema, type Profile, ProfileSchema } from '../domain/index.js';
import { buildInvocation } from '../launchers/index.js';
import { runCommand, runInteractive } from '../process/run.js';
import { sanitizeForTerminal } from '../security/sanitize.js';

export const DEFAULT_CLASSIFICATIONS = ['public', 'personal', 'corporate', 'confidential'];

const list = (v: string) =>
  v
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);

function isInteractive(): boolean {
  return Boolean(process.stdin.isTTY && process.stdout.isTTY);
}

function cancelled<T>(v: T): Exclude<T, symbol> {
  if (p.isCancel(v)) {
    p.cancel('Cancelled.');
    process.exit(1);
  }
  return v as Exclude<T, symbol>;
}

interface AddOptions {
  id?: string;
  app?: string;
  label?: string;
  accepts?: string[];
  tags?: string[];
  launcher?: string;
  distro?: string;
  executable?: string;
  configDir?: string;
  useDefaultLogin?: boolean;
  auth?: string;
  apiKeyEnv?: string;
  mcpConfig?: string;
  provides?: string[];
  model?: string;
  preference?: string;
  routing?: string;
  mcp?: string;
  alias?: string[];
}

const collect = (v: string, prev: string[] = []) => [...prev, v];

/** `--alias jira=ticket,incidencia` (repeatable) → { jira: ['ticket', 'incidencia'] }. */
export function parseAliases(items: readonly string[] = []): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const item of items) {
    const [tool, words] = item.split('=');
    if (!tool || !words) throw new Error(`--alias must look like tool=word1,word2 (got "${item}")`);
    out[tool.trim()] = [...(out[tool.trim()] ?? []), ...list(words)];
  }
  return out;
}

async function promptMissing(o: AddOptions): Promise<AddOptions> {
  p.intro('New Hilvan profile');
  const app =
    o.app ??
    cancelled(
      await p.select({
        message: 'Agent application',
        options: [
          { value: 'claude-code', label: 'Claude Code' },
          { value: 'codex', label: 'Codex CLI' },
        ],
      }),
    );
  const id =
    o.id ??
    cancelled(
      await p.text({
        message: 'Profile id (lowercase, e.g. claude-personal)',
        validate: (v) => (/^[a-z0-9][a-z0-9-]{0,62}$/.test(v ?? '') ? undefined : 'Invalid id'),
      }),
    );
  const label = o.label ?? cancelled(await p.text({ message: 'Label', initialValue: id }));
  const accepts =
    o.accepts ??
    cancelled(
      await p.multiselect({
        message: 'Data classifications this profile may receive',
        options: DEFAULT_CLASSIFICATIONS.map((c) => ({ value: c, label: c })),
        required: true,
      }),
    );
  let useDefaultLogin = o.useDefaultLogin;
  if (!o.configDir && useDefaultLogin === undefined) {
    const adapter = adapterFor(AppSchema.parse(app));
    useDefaultLogin = cancelled(
      await p.confirm({
        message: `Reuse the existing login in ~/${adapter.defaultConfigDir}? (No = new isolated directory)`,
        initialValue: false,
      }),
    );
  }
  return { ...o, app, id, label, accepts, ...(useDefaultLogin ? { useDefaultLogin } : {}) };
}

export function buildProfile(o: AddOptions): Profile {
  const app = AppSchema.parse(o.app);
  if (!o.id) throw new Error('--id is required');
  const launcher = o.launcher ?? 'native';
  let configDir: string | null | undefined = o.configDir;
  if (!configDir) {
    if (launcher !== 'native') {
      throw new Error(
        '--config-dir is required for non-native destinations (path in that environment)',
      );
    }
    configDir = o.useDefaultLogin ? null : defaultConfigDir(o.id);
  }
  if (launcher === 'native' && configDir) configDir = expandHome(configDir);

  const auth =
    (o.auth ?? 'subscription') === 'api-key'
      ? { mode: 'api-key' as const, apiKeyEnv: o.apiKeyEnv ?? '' }
      : { mode: 'subscription' as const };

  return ProfileSchema.parse({
    id: o.id,
    label: o.label ?? o.id,
    app,
    auth,
    configDir,
    destination: {
      launcher,
      ...(o.distro ? { distro: o.distro } : {}),
      ...(o.executable ? { executable: o.executable } : {}),
    },
    accepts: o.accepts ?? [],
    tags: o.tags ?? [],
    tools: {
      mcp: o.mcp ?? (o.mcpConfig ? 'profile' : 'account'),
      ...(o.mcpConfig ? { mcpConfig: o.mcpConfig } : {}),
      provides: o.provides ?? [],
      aliases: parseAliases(o.alias),
    },
    routing: o.routing ?? 'default',
    model: o.model ?? null,
    preference: o.preference ? Number(o.preference) : 0,
  });
}

function checkDeps(): CheckDeps {
  return {
    host: currentHost(),
    run: (inv) =>
      runCommand(inv.command, inv.args, {
        env: inv.env,
        timeoutMs: 20_000,
        ...(inv.cwd ? { cwd: inv.cwd } : {}),
      }),
    configDirIssues: (dir) => configDirIssues(dir),
  };
}

const sanitizeName = (n: string) => sanitizeForTerminal(n, { multiline: false, maxLength: 80 });

/** Human label for a profile's config dir. */
export function dirLabel(p: Profile): string {
  return p.configDir ?? `CLI default login (~/${adapterFor(p.app).defaultConfigDir})`;
}

export function formatCheck(c: ProfileCheck): string {
  const icon = c.issues.length ? '✖' : c.warnings.length ? '!' : '✔';
  const who = c.auth.loggedIn
    ? `${c.auth.account ?? 'account not reported'}${c.auth.method ? ` (${c.auth.method})` : ''}`
    : 'not logged in';
  const lines = [`${icon} ${c.profileId}  [${c.app} @ ${c.destination}]  ${who}`];
  for (const i of c.issues) lines.push(`    ✖ ${i}`);
  for (const w of c.warnings) lines.push(`    ! ${w}`);
  return lines.join('\n');
}

function fail(err: unknown): never {
  const msg = err instanceof z.ZodError ? z.prettifyError(err) : (err as Error).message;
  console.error(`✖ ${msg}`);
  process.exit(1);
}

export function registerProfileCommands(program: Command): void {
  const profile = program
    .command('profile')
    .description('Manage agent profiles (one per account and environment).');

  profile
    .command('add')
    .description('Register a profile. Prompts for missing values when run in a terminal.')
    .option('--id <id>', 'stable id, e.g. claude-personal')
    .option('--app <app>', 'claude-code | codex')
    .option('--label <label>', 'display name')
    .option(
      '--accepts <list>',
      `classifications it may receive (${DEFAULT_CLASSIFICATIONS.join(',')})`,
      list,
    )
    .option('--tags <list>', 'tags for policy rules, e.g. personal', list)
    .option('--launcher <launcher>', 'native | wsl | windows', 'native')
    .option('--distro <name>', 'WSL distribution (wsl launcher)')
    .option('--executable <path>', 'override the CLI executable')
    .option('--config-dir <dir>', 'isolated CLI config dir (default ~/.hilvan-profiles/<id>)')
    .option('--use-default-login', "reuse the CLI's default directory (~/.claude or ~/.codex)")
    .option('--auth <mode>', 'subscription | api-key', 'subscription')
    .option('--api-key-env <VAR>', 'environment variable holding the API key (api-key mode)')
    .option(
      '--mcp <mode>',
      'account (its own MCP servers, default) | profile (only --mcp-config) | none',
    )
    .option('--mcp-config <file>', 'MCP servers for this profile (implies --mcp profile)')
    .option('--routing <role>', 'default | reserved (only for tasks that need its tools)')
    .option(
      '--alias <tool=words>',
      'words that reveal a tool is needed, e.g. jira=ticket,incidencia (repeatable)',
      collect,
    )
    .option('--provides <list>', 'declared capabilities/tools, e.g. jira,confluence', list)
    .option('--model <model>', 'default model')
    .option('--preference <n>', 'tie-breaker among eligible profiles (higher first)')
    .action(async (opts: AddOptions) => {
      try {
        const needsPrompt = !opts.id || !opts.app || !opts.accepts;
        const filled = needsPrompt && isInteractive() ? await promptMissing(opts) : opts;
        if (!filled.id || !filled.app || !filled.accepts?.length) {
          throw new Error('--id, --app and --accepts are required in non-interactive mode');
        }
        const prof = buildProfile(filled);
        const file = profilesFile();
        await saveProfiles(file, addProfile(await loadProfiles(file), prof));
        console.log(`✔ profile ${prof.id} added (${prof.app} @ ${describeDestination(prof)})`);
        console.log(`  config dir: ${dirLabel(prof)}`);
        console.log(
          `  next: hilvan profile login ${prof.id}   then: hilvan profile check ${prof.id}`,
        );
        if (isInteractive() && !opts.id) p.outro('Done.');
      } catch (err) {
        fail(err);
      }
    });

  profile
    .command('set <id>')
    .description('Change routing, MCP mode, tools or aliases of an existing profile.')
    .option('--routing <role>', 'default | reserved')
    .option('--mcp <mode>', 'account | profile | none')
    .option('--mcp-config <file>', 'MCP servers file for --mcp profile')
    .option('--provides <list>', 'replace the declared tools, e.g. jira,confluence', list)
    .option(
      '--alias <tool=words>',
      'add aliases, e.g. jira=ticket,incidencia (repeatable)',
      collect,
    )
    .option('--accepts <list>', 'replace the accepted classifications', list)
    .option('--preference <n>', 'tie-breaker among eligible profiles')
    .action(
      async (
        id: string,
        o: {
          routing?: string;
          mcp?: string;
          mcpConfig?: string;
          provides?: string[];
          alias?: string[];
          accepts?: string[];
          preference?: string;
        },
      ) => {
        try {
          const file = profilesFile();
          const data = await loadProfiles(file);
          const cur = findProfile(data, id);
          const aliases = { ...cur.tools.aliases };
          for (const [tool, words] of Object.entries(parseAliases(o.alias))) {
            aliases[tool] = [...new Set([...(aliases[tool] ?? []), ...words])];
          }
          const next = ProfileSchema.parse({
            ...cur,
            ...(o.routing ? { routing: o.routing } : {}),
            ...(o.accepts ? { accepts: o.accepts } : {}),
            ...(o.preference ? { preference: Number(o.preference) } : {}),
            tools: {
              ...cur.tools,
              ...(o.mcp ? { mcp: o.mcp } : {}),
              ...(o.mcpConfig ? { mcpConfig: o.mcpConfig } : {}),
              ...(o.provides ? { provides: o.provides } : {}),
              aliases,
            },
          });
          await saveProfiles(file, {
            ...data,
            profiles: data.profiles.map((p) => (p.id === id ? next : p)),
          });
          console.log(
            `✔ ${id}: routing ${next.routing} · mcp ${next.tools.mcp} · provides ${next.tools.provides.join(', ') || '—'}`,
          );
        } catch (err) {
          fail(err);
        }
      },
    );

  profile
    .command('tools <id>')
    .description(
      "List the MCP servers the account itself has configured and compare with 'provides'.",
    )
    .option('--apply', "add discovered servers to the profile's provides")
    .action(async (id: string, o: { apply?: boolean }) => {
      try {
        const file = profilesFile();
        const data = await loadProfiles(file);
        const prof = findProfile(data, id);
        const d = await discoverTools(prof, checkDeps());
        if (d.error) throw new Error(`could not list MCP servers: ${d.error}`);
        if (!d.discovered.length) console.log(`${id}: no MCP servers configured in this account`);
        for (const t of d.discovered) {
          const mark = prof.tools.provides.includes(t.id) ? '✔' : '+';
          console.log(`${mark} ${t.id}  (${sanitizeName(t.name)}, ${t.status})`);
        }
        for (const s of d.stale)
          console.log(`? ${s}  declared in provides but not configured in the account`);
        if (prof.tools.mcp !== 'account' && d.discovered.length) {
          console.log(
            `! mcp mode is "${prof.tools.mcp}": these account servers are NOT loaded in Hilvan runs`,
          );
        }
        if (o.apply && d.missing.length) {
          const next = ProfileSchema.parse({
            ...prof,
            tools: { ...prof.tools, provides: [...prof.tools.provides, ...d.missing] },
          });
          await saveProfiles(file, {
            ...data,
            profiles: data.profiles.map((p) => (p.id === id ? next : p)),
          });
          console.log(`✔ added to provides: ${d.missing.join(', ')}`);
        } else if (d.missing.length) {
          console.log(
            `  next: hilvan profile tools ${id} --apply   (then: hilvan profile set ${id} --routing reserved)`,
          );
        }
      } catch (err) {
        fail(err);
      }
    });

  profile
    .command('list')
    .description('List configured profiles.')
    .option('--json', 'print as JSON')
    .action(async (opts: { json?: boolean }) => {
      const { profiles } = await loadProfiles(profilesFile()).catch(fail);
      if (opts.json) return console.log(JSON.stringify(profiles, null, 2));
      if (!profiles.length) return console.log('No profiles yet. Add one with: hilvan profile add');
      const rows = profiles.map((x) => [
        x.id,
        x.app,
        describeDestination(x),
        x.auth.mode,
        x.accepts.join(','),
        dirLabel(x),
      ]);
      const head = ['ID', 'APP', 'DESTINATION', 'AUTH', 'ACCEPTS', 'CONFIG DIR'];
      const widths = head.map((h, i) =>
        Math.max(h.length, ...rows.map((r) => (r[i] ?? '').length)),
      );
      for (const r of [head, ...rows])
        console.log(
          r
            .map((c, i) => c.padEnd(widths[i] ?? 0))
            .join('  ')
            .trimEnd(),
        );
    });

  profile
    .command('show <id>')
    .description('Show a profile as YAML.')
    .action(async (id: string) => {
      try {
        console.log(stringify(findProfile(await loadProfiles(profilesFile()), id)).trimEnd());
      } catch (err) {
        fail(err);
      }
    });

  profile
    .command('remove <id>')
    .description('Remove a profile from Hilvan. Its CLI config dir and login are left untouched.')
    .option('-y, --yes', 'do not ask for confirmation')
    .action(async (id: string, opts: { yes?: boolean }) => {
      try {
        const file = profilesFile();
        const data = await loadProfiles(file);
        const prof = findProfile(data, id);
        if (!opts.yes && isInteractive()) {
          const ok = cancelled(
            await p.confirm({ message: `Remove profile ${id}? (${dirLabel(prof)} is kept)` }),
          );
          if (!ok) return;
        }
        await saveProfiles(file, removeProfile(data, id));
        console.log(`✔ profile ${id} removed; ${dirLabel(prof)} was not deleted`);
      } catch (err) {
        fail(err);
      }
    });

  profile
    .command('login <id>')
    .description("Open the official CLI login for this profile's isolated config dir.")
    .action(async (id: string) => {
      try {
        const prof = findProfile(await loadProfiles(profilesFile()), id);
        const host = currentHost();
        if (prof.destination.launcher === 'native' && prof.configDir) {
          await ensureConfigDir(prof.configDir);
        }
        const adapter = adapterFor(prof.app);
        const inv = buildInvocation(prof.destination, adapter.loginCommand(prof, host.env), host);
        console.log(`→ ${prof.app} login for ${id} (${dirLabel(prof)})`);
        const code = await runInteractive(inv.command, inv.args, { env: inv.env });
        if (code !== 0) throw new Error(`login exited with ${code ?? 'an error'}`);
        console.log(formatCheck(await checkProfile(prof, checkDeps())));
      } catch (err) {
        fail(err);
      }
    });

  profile
    .command('check [id]')
    .description('Verify executable, config dir and active account of one or all profiles.')
    .option('--json', 'print as JSON')
    .action(async (id: string | undefined, opts: { json?: boolean }) => {
      try {
        const data = await loadProfiles(profilesFile());
        const targets = id ? [findProfile(data, id)] : data.profiles;
        if (!targets.length)
          return console.log('No profiles yet. Add one with: hilvan profile add');
        const deps = checkDeps();
        const checks = await Promise.all(targets.map((t) => checkProfile(t, deps)));
        const shared = sharedAccountWarnings(checks);
        if (opts.json) console.log(JSON.stringify({ checks, warnings: shared }, null, 2));
        else {
          for (const c of checks) console.log(formatCheck(c));
          for (const w of shared) console.log(`! ${w}`);
        }
        if (checks.some((c) => c.issues.length)) process.exitCode = 1;
      } catch (err) {
        fail(err);
      }
    });
}
