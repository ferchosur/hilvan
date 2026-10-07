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
    tools: { ...(o.mcpConfig ? { mcpConfig: o.mcpConfig } : {}), provides: o.provides ?? [] },
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
    .option('--mcp-config <file>', 'MCP servers for this profile (Claude Code)')
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
