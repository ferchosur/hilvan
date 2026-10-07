import { readFile, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import type { Command } from 'commander';
import { profilesFile } from '../config/paths.js';
import { defaultPolicyPaths, loadEffectivePolicy } from '../config/policy.js';
import { loadProfiles } from '../config/profiles.js';
import { approveHandoff, cancelHandoff, draftHandoff, sha256 } from '../core/handoff.js';
import { findProfile } from '../core/profiles.js';
import { runInteractive } from '../process/run.js';
import { sanitizeForTerminal } from '../security/sanitize.js';
import { ProjectStore } from '../store/project.js';

function fail(err: unknown): never {
  console.error(`✖ ${(err as Error).message}`);
  process.exit(1);
}

/** Opens the user's editor on a private temp file (no shell; EDITOR may carry arguments). */
async function editInEditor(stateDir: string, text: string): Promise<string> {
  const file = path.join(stateDir, `handoff-edit-${process.pid}.md`);
  await writeFile(file, text, { mode: 0o600 });
  try {
    const editor = (
      process.env.VISUAL ||
      process.env.EDITOR ||
      (process.platform === 'win32' ? 'notepad' : 'vi')
    ).trim();
    const [cmd = 'vi', ...args] = editor.split(/\s+/);
    const code = await runInteractive(cmd, [...args, file]);
    if (code !== 0) throw new Error(`editor exited with ${code}`);
    return await readFile(file, 'utf8');
  } finally {
    await rm(file, { force: true });
  }
}

export function registerHandoffCommand(program: Command): void {
  program
    .command('handoff <task>')
    .description(
      'Hand a task over to another profile with a reviewable continuity package (you approve exactly what is sent).',
    )
    .requiredOption('--to <profile>', 'profile that continues the task')
    .option('--edit', 'edit the package in $EDITOR before approving')
    .option('--approve', 'approve without prompting (non-interactive)')
    .option('--print', 'only print the draft package; send nothing')
    .action(
      async (
        ref: string,
        o: { to: string; edit?: boolean; approve?: boolean; print?: boolean },
      ) => {
        try {
          const store = await ProjectStore.find(process.cwd());
          const taskId = await store.resolveTaskId(ref);
          const profiles = await loadProfiles(profilesFile());
          const to = findProfile(profiles, o.to);
          const task = await store.loadTask(taskId);
          const from = profiles.profiles.find((p) => p.id === task.profileId);
          const policy = await loadEffectivePolicy(defaultPolicyPaths(store.root));
          const draft = await draftHandoff(store, taskId, to, from, policy);

          if (!draft.evaluation.eligible) {
            for (const x of draft.evaluation.exclusions)
              console.error(`  ✖ ${x.reason}${x.enforced ? ' (enforced)' : ''}`);
            throw new Error(`${to.id} is not eligible to receive this task; nothing was sent`);
          }
          if (draft.secrets.length) {
            console.log(
              `! ${draft.secrets.length} possible secret(s) were redacted: ${[...new Set(draft.secrets.map((s) => s.kind))].join(', ')}`,
            );
          }
          for (const a of draft.evaluation.approvals)
            console.log(`? policy: ${a.reason} [rule ${a.ruleId}, ${a.layer}]`);

          let body = draft.body;
          if (o.edit) body = await editInEditor(store.dir, body);
          console.log(
            `──── continuity package for ${to.id} (sha256 ${sha256(body).slice(0, 12)}) ────`,
          );
          console.log(sanitizeForTerminal(body));
          console.log('────────────────────────────────────────────────────────');
          if (o.print) return;

          let ok = Boolean(o.approve);
          if (!ok && process.stdin.isTTY) {
            const p = await import('@clack/prompts');
            ok =
              (await p.confirm({
                message: `Send exactly this package to ${to.id}?`,
                initialValue: false,
              })) === true;
          }
          if (!ok) {
            await cancelHandoff(store, draft);
            console.log('✖ cancelled; nothing was sent');
            process.exitCode = 1;
            return;
          }
          const r = await approveHandoff(store, draft, body);
          console.log(
            `✔ hand-off ${draft.id.slice(-6)} approved (sha256 ${r.hash.slice(0, 12)}); task now belongs to ${to.id}`,
          );
          console.log(`  next: hilvan task run ${taskId.slice(-6)} --profile ${to.id}`);
        } catch (err) {
          fail(err);
        }
      },
    );
}
