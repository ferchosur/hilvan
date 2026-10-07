import type { Command } from 'commander';
import { profilesFile } from '../config/paths.js';
import { defaultPolicyPaths, loadEffectivePolicy } from '../config/policy.js';
import { loadProfiles } from '../config/profiles.js';
import { findProfile } from '../core/profiles.js';
import { evaluate } from '../policy/evaluate.js';
import { ProjectStore } from '../store/project.js';

function fail(err: unknown): never {
  console.error(`✖ ${(err as Error).message}`);
  process.exit(1);
}

async function projectRoot(): Promise<string | undefined> {
  try {
    return (await ProjectStore.find(process.cwd())).root;
  } catch {
    return undefined;
  }
}

export function registerPolicyCommands(program: Command): void {
  const policy = program
    .command('policy')
    .description('Inspect the layered data policy (managed → user → project).');

  policy
    .command('show')
    .description('Show the effective policy and where each rule comes from.')
    .option('--json', 'print as JSON')
    .action(async (o: { json?: boolean }) => {
      try {
        const paths = defaultPolicyPaths(await projectRoot());
        const eff = await loadEffectivePolicy(paths);
        if (o.json) return console.log(JSON.stringify({ paths, ...eff }, null, 2));
        console.log(
          `files: managed ${paths.managed} · user ${paths.user}${paths.project ? ` · project ${paths.project}` : ''}`,
        );
        console.log(`classifications: ${eff.classifications.join(' < ')}`);
        if (!eff.rules.length)
          console.log('rules: none (only profile `accepts` and required tools apply)');
        for (const r of eff.rules) {
          const from = JSON.stringify(r.from);
          const to = JSON.stringify(r.to);
          console.log(
            `- [${r.layer}${r.enforced ? ', ENFORCED' : ''}] ${r.id}: ${r.effect} from ${from} to ${to} — ${r.reason}`,
          );
        }
        for (const w of eff.warnings) console.log(`! ${w}`);
      } catch (err) {
        fail(err);
      }
    });

  policy
    .command('check <task>')
    .description('Explain whether a profile may receive a task, and why.')
    .requiredOption('-p, --profile <id>', 'profile to check')
    .action(async (ref: string, o: { profile: string }) => {
      try {
        const store = await ProjectStore.find(process.cwd());
        const task = await store.loadTask(await store.resolveTaskId(ref));
        const profile = findProfile(await loadProfiles(profilesFile()), o.profile);
        const ev = evaluate(
          await loadEffectivePolicy(defaultPolicyPaths(store.root)),
          task,
          profile,
        );
        if (ev.eligible) {
          console.log(
            `✔ ${profile.id} is eligible for ${task.classification} data${ev.requiresApproval ? ' — approval required' : ''}`,
          );
        } else {
          console.log(`✖ ${profile.id} is not eligible`);
        }
        for (const x of ev.exclusions)
          console.log(`  ✖ ${x.reason} [${x.source}${x.enforced ? ', enforced' : ''}]`);
        for (const a of ev.approvals) console.log(`  ? ${a.reason} [rule ${a.ruleId}, ${a.layer}]`);
        if (!ev.eligible) process.exitCode = 1;
      } catch (err) {
        fail(err);
      }
    });
}
