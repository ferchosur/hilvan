import { readFileSync } from 'node:fs';
import { Command } from 'commander';

// Same relative location from src/cli (tsx) and dist/cli (build).
const pkg = JSON.parse(readFileSync(new URL('../../package.json', import.meta.url), 'utf8')) as {
  version: string;
};

type Register = (program: Command) => void;

/**
 * Command groups are loaded lazily: only the group being run is imported, so
 * `hilvan criterio ask` never loads YAML, prompts or adapters (NFR-013, startup budget).
 */
const GROUPS: Record<string, () => Promise<Register>> = {
  doctor: async () => (await import('./doctor-command.js')).registerDoctorCommand,
  profile: async () => (await import('./profile.js')).registerProfileCommands,
  criterio: async () => (await import('./criterio.js')).registerCriterioCommands,
  audit: async () => (await import('./audit.js')).registerAuditCommands,
  policy: async () => (await import('./policy.js')).registerPolicyCommands,
  init: async () => (await import('./task.js')).registerInitCommand,
  task: async () => (await import('./task.js')).registerTaskCommands,
  handoff: async () => (await import('./handoff.js')).registerHandoffCommand,
  review: async () => (await import('./review.js')).registerReviewCommands,
  accept: async () => (await import('./review.js')).registerReviewCommands,
  stats: async () => (await import('./stats.js')).registerStatsCommand,
};

export function selectGroups(argv: readonly string[]): string[] {
  const args = argv.slice(2);
  const sub = args.find((a) => !a.startsWith('-'));
  if (sub && sub in GROUPS) return [sub];
  const versionOnly = args.length > 0 && args.every((a) => a === '--version' || a === '-V');
  return versionOnly ? [] : Object.keys(GROUPS);
}

export async function run(argv: readonly string[] = process.argv): Promise<void> {
  const program = new Command()
    .name('hilvan')
    .description('Coordinate AI coding agents across providers and accounts.')
    .version(pkg.version);
  // Several command names can share one module (review/accept): register each module once.
  const registered = new Set<Register>();
  for (const name of selectGroups(argv)) {
    const load = GROUPS[name];
    if (!load) continue;
    const register = await load();
    if (registered.has(register)) continue;
    registered.add(register);
    register(program);
  }
  await program.parseAsync([...argv]);
}
