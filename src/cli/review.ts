import type { Command } from 'commander';
import { criterioFile, loadCriterio, saveCriterio } from '../config/criterio-store.js';
import { profilesFile, userConfigDir } from '../config/paths.js';
import { defaultPolicyPaths, loadEffectivePolicy } from '../config/policy.js';
import { loadProfiles } from '../config/profiles.js';
import { currentHost, findProfile } from '../core/profiles.js';
import { acceptTask, crossReview, learnOutcome, reviewDiff } from '../core/review.js';
import { discardTask } from '../core/tasks.js';
import { isSensitivePath } from '../criterio/features.js';
import { sanitizeForTerminal } from '../security/sanitize.js';
import { ProjectStore } from '../store/project.js';

function fail(err: unknown): never {
  console.error(`✖ ${(err as Error).message}`);
  process.exit(1);
}

export function registerReviewCommands(program: Command): void {
  program
    .command('review <task>')
    .description(
      'Show the task diff with Criterio risk; with --by, ask another profile for a read-only review.',
    )
    .option('--by <profile>', 'cross review with this profile (read-only)')
    .option('--stat', 'only the list of changed files')
    .option('--approve', 'approve sending the task to the reviewer if a policy rule requires it')
    .action(async (ref: string, o: { by?: string; stat?: boolean; approve?: boolean }) => {
      try {
        const store = await ProjectStore.find(process.cwd());
        const taskId = await store.resolveTaskId(ref);
        const { engine } = await loadCriterio(criterioFile(userConfigDir()));
        if (o.by) {
          const reviewer = findProfile(await loadProfiles(profilesFile()), o.by);
          console.log(`→ cross review of ${taskId.slice(-6)} by ${reviewer.id} (read-only)`);
          const r = await crossReview(store, taskId, reviewer, {
            host: currentHost(),
            policy: await loadEffectivePolicy(defaultPolicyPaths(store.root)),
            approve: async (ev, brief) => {
              for (const a of ev.approvals) console.log(`? ${a.reason} [rule ${a.ruleId}]`);
              console.log(sanitizeForTerminal(brief));
              if (o.approve) return true;
              if (!process.stdin.isTTY) return false;
              const p = await import('@clack/prompts');
              return (
                (await p.confirm({
                  message: `Send it to ${reviewer.id}?`,
                  initialValue: false,
                })) === true
              );
            },
          });
          console.log(sanitizeForTerminal(r.text));
          const icon =
            r.verdict === 'approve' ? '✔' : r.verdict === 'changes_requested' ? '✖' : '?';
          console.log(`\n${icon} verdict: ${r.verdict} — saved in ${r.file}`);
          return;
        }
        const { task, diff, truncated } = await reviewDiff(store, taskId);
        const last = task.runs.at(-1);
        console.log(
          `${task.id}  [${task.state}]  ${sanitizeForTerminal(task.title, { multiline: false })}`,
        );
        if (!task.branch) {
          console.log('read-only task: no diff. Last agent summary (unverified):');
          console.log(sanitizeForTerminal(last?.summary ?? '(none)'));
        } else {
          if (last?.diff) {
            const risk = engine.diffRisk(last.diff);
            console.log(`risk ${risk.value}/10 · ${risk.why.map((w) => w.signal).join(' · ')}`);
            for (const p of last.diff.paths) {
              console.log(
                `  ${isSensitivePath(p) ? '⚠' : '·'} ${sanitizeForTerminal(p, { multiline: false })}`,
              );
            }
            if (
              engine.dispose(risk, 'suggest_review', risk.value) === 'escalate' &&
              !task.reviews.length
            ) {
              console.log(
                `  ! high risk: consider \`hilvan review ${task.id.slice(-6)} --by <other-profile>\` before accepting`,
              );
            }
          }
          if (!o.stat)
            console.log(`\n${sanitizeForTerminal(diff)}${truncated ? '\n… (diff truncated)' : ''}`);
        }
        for (const r of task.reviews) console.log(`review by ${r.profileId}: ${r.verdict}`);
        if (task.state === 'in_review') {
          console.log(
            `\nnext: hilvan accept ${task.id.slice(-6)}   or   hilvan task discard ${task.id.slice(-6)} --yes`,
          );
        }
      } catch (err) {
        fail(err);
      }
    });

  program
    .command('accept <task>')
    .description(
      'Accept a reviewed task: merge its branch into the base branch and clean up (Criterio learns).',
    )
    .action(async (ref: string) => {
      try {
        const store = await ProjectStore.find(process.cwd());
        const file = criterioFile(userConfigDir());
        const { engine } = await loadCriterio(file);
        const t = await acceptTask(store, await store.resolveTaskId(ref), engine);
        await saveCriterio(file, engine);
        console.log(
          `✔ task ${t.id.slice(-6)} accepted${t.mode === 'write' ? ` and merged into ${t.baseBranch}` : ''}`,
        );
      } catch (err) {
        fail(err);
      }
    });
}

/** `task discard` learning hook, shared with the task commands. */
export async function discardWithLearning(store: ProjectStore, taskId: string) {
  const file = criterioFile(userConfigDir());
  const { engine } = await loadCriterio(file);
  const t = await discardTask(store, taskId, undefined, (task) =>
    learnOutcome(task, false, engine),
  );
  await saveCriterio(file, engine);
  return t;
}
