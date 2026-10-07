import { readFile } from 'node:fs/promises';
import type { Command } from 'commander';
import type { AgentEvent } from '../adapters/index.js';
import { criterioFile, loadCriterio, saveCriterio } from '../config/criterio-store.js';
import { profilesFile, userConfigDir } from '../config/paths.js';
import { defaultPolicyPaths, loadEffectivePolicy } from '../config/policy.js';
import { loadProfiles } from '../config/profiles.js';
import { currentHost, describeDestination, findProfile } from '../core/profiles.js';
import { recommend } from '../core/recommend.js';
import { repoAgentConfig, trust, untrusted } from '../core/repo-trust.js';
import {
  type Brief,
  cancelTask,
  createTask,
  discardTask,
  recoverStaleRun,
  runTask,
  suggest,
} from '../core/tasks.js';
import { TASK_TYPES, type TaskType } from '../criterio/index.js';
import type { RunSummary, Task } from '../domain/index.js';
import { currentBranch, repoRoot } from '../gitops/index.js';
import { sanitizeForTerminal } from '../security/sanitize.js';
import { ProjectStore } from '../store/project.js';

const CLASSIFICATIONS = ['public', 'personal', 'corporate', 'confidential'];
const list = (v: string) =>
  v
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
const collect = (v: string, prev: string[] = []) => [...prev, v];
const short = (id: string) => id.slice(-6);
const clean = (s: string, max = 200) =>
  sanitizeForTerminal(s, { multiline: false, maxLength: max });

function fail(err: unknown): never {
  console.error(`✖ ${(err as Error).message}`);
  process.exit(1);
}

function pct(x: number) {
  return `${Math.round(x * 100)} %`;
}

export function formatRun(r: RunSummary): string {
  const parts = [
    `${r.state}${r.blockedReason ? ` (${r.blockedReason})` : ''}`,
    `profile ${r.profileId}`,
  ];
  if (r.usage?.costUsd !== undefined) parts.push(`~$${r.usage.costUsd.toFixed(4)} estimated`);
  if (r.usage?.inputTokens !== undefined) {
    parts.push(`${r.usage.inputTokens} in / ${r.usage.outputTokens ?? 0} out tokens`);
  }
  if (r.limits?.length)
    parts.push(`limits ${r.limits.map((w) => `${w.name} ${pct(w.utilization)}`).join(', ')}`);
  if (r.diff)
    parts.push(`${r.diff.filesChanged} files +${r.diff.linesAdded} −${r.diff.linesDeleted}`);
  if (r.risk !== undefined) parts.push(`risk ${r.risk}/10`);
  if (r.meetsCriteria !== undefined) parts.push(`meets brief ~${pct(r.meetsCriteria)}`);
  return parts.join(' · ');
}

function printEvent(e: AgentEvent): void {
  switch (e.type) {
    case 'session':
      console.log(
        `▸ session ${clean(e.sessionId, 60)}${e.model ? ` (${clean(e.model, 40)})` : ''}`,
      );
      break;
    case 'message':
      console.log(
        sanitizeForTerminal(e.text, { maxLength: 4000 })
          .split('\n')
          .map((l) => `  │ ${l}`)
          .join('\n'),
      );
      break;
    case 'tool':
      if (e.status !== 'completed') {
        console.log(
          `  · ${clean(e.name, 40)} ${e.status}${e.summary ? `: ${clean(e.summary, 120)}` : ''}`,
        );
      }
      break;
    case 'retry':
      console.log(`  ! retry ${e.attempt}: ${clean(e.error, 80)}`);
      break;
    default:
      break;
  }
}

async function openStore(): Promise<ProjectStore> {
  const store = await ProjectStore.find(process.cwd());
  const recovered = await recoverStaleRun(store);
  if (recovered)
    console.error(`! a previous run of ${short(recovered)} was interrupted; task marked uncertain`);
  return store;
}

export function registerInitCommand(program: Command): void {
  program
    .command('init')
    .description('Make the current git repository a Hilvan project (.hilvan/, ignored by git).')
    .option('--base <branch>', 'base branch for task worktrees (default: current branch)')
    .option('--setup <cmd>', 'command run in each new worktree, e.g. "npm ci"')
    .action(async (o: { base?: string; setup?: string }) => {
      try {
        const root = await repoRoot(process.cwd());
        const store = new ProjectStore(root);
        const base = o.base ?? (await currentBranch(root));
        const name = root.split(/[\\/]/).pop() ?? 'project';
        await store.init({
          name,
          baseBranch: base,
          ...(o.setup ? { setupHook: o.setup.split(' ') } : {}),
        });
        await store.audit({ type: 'project.initialised', data: { baseBranch: base } });
        console.log(`✔ Hilvan project initialised in ${root}/.hilvan (base branch: ${base})`);
        console.log('  next: hilvan task new "<title>" --objective "…" --criteria "…"');
      } catch (err) {
        fail(err);
      }
    });
}

export function registerTaskCommands(program: Command): void {
  const task = program.command('task').description('Create, run and inspect tasks.');

  task
    .command('new <title>')
    .description(
      'Create a task from a brief. Criterio suggests type, complexity and classification.',
    )
    .option('--objective <text>', 'what should be achieved')
    .option('--criteria <text>', 'acceptance criterion (repeatable)', collect)
    .option('--constraint <text>', 'constraint (repeatable)', collect)
    .option('--classification <c>', CLASSIFICATIONS.join(' | '), 'personal')
    .option('--mode <mode>', 'write (own worktree) | read', 'write')
    .option('--requires <list>', 'required tools/capabilities, e.g. jira', list)
    .option('--type <type>', TASK_TYPES.join(' | '))
    .option('--brief-file <file>', 'read the objective from a Markdown file')
    .action(
      async (
        title: string,
        o: {
          objective?: string;
          criteria?: string[];
          constraint?: string[];
          classification: string;
          mode: string;
          requires?: string[];
          type?: string;
          briefFile?: string;
        },
      ) => {
        try {
          if (o.mode !== 'write' && o.mode !== 'read')
            throw new Error('--mode must be write or read');
          if (o.type && !(TASK_TYPES as readonly string[]).includes(o.type)) {
            throw new Error(`--type must be one of ${TASK_TYPES.join(', ')}`);
          }
          const store = await openStore();
          const objective = o.briefFile ? await readFile(o.briefFile, 'utf8') : o.objective;
          const brief: Brief = {
            title,
            ...(objective ? { objective } : {}),
            ...(o.criteria ? { criteria: o.criteria } : {}),
            ...(o.constraint ? { constraints: o.constraint } : {}),
            classification: o.classification,
            mode: o.mode,
            ...(o.requires ? { requires: o.requires } : {}),
          };
          const file = criterioFile(userConfigDir());
          const { engine } = await loadCriterio(file);
          const s = suggest(engine, brief);
          const explicitType = o.type as TaskType | undefined;
          const typeDisp = engine.dispose(s.taskType, 'label');
          const chosenType = explicitType ?? (typeDisp === 'auto' ? s.taskType.value : undefined);
          const created = await createTask(
            store,
            {
              ...brief,
              ...(chosenType ? { taskType: chosenType } : {}),
              complexity: s.complexity.value,
            },
            explicitType ? engine : undefined,
          );
          if (explicitType) await saveCriterio(file, engine);

          console.log(`✔ task ${short(created.id)} created: ${clean(title)}`);
          console.log(
            `  criterio: type ${s.taskType.value} (p=${s.taskType.p.toFixed(2)}${explicitType ? `, you chose ${explicitType}` : `, ${typeDisp}`}) · complexity ${s.complexity.value}/10`,
          );
          if (s.classification.value) {
            console.log(
              `  ! Criterio suggests classification "${s.classification.value}" (${s.classification.why
                .slice(0, 3)
                .map((r) => r.signal)
                .join(
                  ', ',
                )}). Re-create with --classification ${s.classification.value} if it applies.`,
            );
          }
          console.log(`  next: hilvan task run ${short(created.id)} --profile <id>`);
        } catch (err) {
          fail(err);
        }
      },
    );

  task
    .command('list')
    .description('List tasks of this project.')
    .option('--json', 'print as JSON')
    .action(async (o: { json?: boolean }) => {
      try {
        const tasks = await (await openStore()).listTasks();
        if (o.json) return console.log(JSON.stringify(tasks, null, 2));
        if (!tasks.length)
          return console.log('No tasks yet. Create one with: hilvan task new "<title>"');
        for (const t of tasks) {
          console.log(
            `${short(t.id)}  ${t.state.padEnd(11)} ${(t.taskType ?? '—').padEnd(9)} ${t.classification.padEnd(12)} ${(t.profileId ?? '—').padEnd(16)} ${clean(t.title, 60)}`,
          );
        }
      } catch (err) {
        fail(err);
      }
    });

  task
    .command('show <id>')
    .description('Show a task, its brief and its runs.')
    .action(async (ref: string) => {
      try {
        const store = await openStore();
        const t = await store.loadTask(await store.resolveTaskId(ref));
        printTask(t);
        console.log(`\n${sanitizeForTerminal(await store.brief(t.id))}`);
      } catch (err) {
        fail(err);
      }
    });

  task
    .command('log <id>')
    .description('Print the stored agent events of the last run (secrets redacted).')
    .option('--run <runId>', 'a specific run')
    .action(async (ref: string, o: { run?: string }) => {
      try {
        const store = await openStore();
        const t = await store.loadTask(await store.resolveTaskId(ref));
        const runId = o.run ?? t.runs[t.runs.length - 1]?.id;
        if (!runId) return console.log('No runs yet.');
        const text = await readFile(store.runFile(t.id, runId), 'utf8').catch(() => '');
        process.stdout.write(sanitizeForTerminal(text));
      } catch (err) {
        fail(err);
      }
    });

  task
    .command('run <id>')
    .description('Run a task with a profile; streams the agent output and records the run.')
    .requiredOption('-p, --profile <id>', 'profile that runs the task')
    .option('--timeout <minutes>', 'cancel the run after this many minutes', '60')
    .option(
      '--trust-repo-hooks',
      'accept hooks defined by this repository (.claude/settings*.json)',
    )
    .option(
      '--approve',
      'approve sending the brief when a policy rule requires approval (non-interactive)',
    )
    .action(
      async (
        ref: string,
        o: { profile: string; timeout: string; trustRepoHooks?: boolean; approve?: boolean },
      ) => {
        try {
          const store = await openStore();
          const taskId = await store.resolveTaskId(ref);
          const profile = findProfile(await loadProfiles(profilesFile()), o.profile);
          const { engine } = await loadCriterio(criterioFile(userConfigDir()));
          if (profile.app === 'claude-code') {
            // SR-012: repository hooks run without a trust dialog in `claude -p`.
            const pending = await untrusted(store.dir, await repoAgentConfig(store.root));
            if (pending.length && !o.trustRepoHooks) {
              for (const i of pending) console.error(`! ${i.file} ${i.risk}`);
              throw new Error(
                'review those hooks; if you trust them, re-run with --trust-repo-hooks (remembered until the file changes)',
              );
            }
            if (pending.length) {
              await trust(store.dir, pending);
              await store.audit({
                type: 'repo_config.trusted',
                data: { files: pending.map((i) => `${i.file}:${i.sha256.slice(0, 12)}`) },
              });
            }
          }
          console.log(
            `→ running ${short(taskId)} with ${profile.id} (${profile.app} @ ${profile.destination.launcher})`,
          );
          let cancel: (() => void) | undefined;
          const onSigint = () => {
            console.error('\n! cancelling… (Ctrl+C again to force)');
            if (cancel) cancel();
            process.removeListener('SIGINT', onSigint);
          };
          process.on('SIGINT', onSigint);
          const policy = await loadEffectivePolicy(defaultPolicyPaths(store.root));
          for (const w of policy.warnings) console.error(`! policy: ${w}`);
          const report = await runTask(store, taskId, profile, {
            host: currentHost(),
            policy,
            approve: async (ev, brief) => {
              console.log(`\n? ${profile.id} needs your approval to receive this task:`);
              for (const a of ev.approvals) {
                console.log(`  · ${clean(a.reason, 200)} [rule ${a.ruleId}, ${a.layer}]`);
              }
              console.log('──── exactly what will be sent (brief) ────');
              console.log(sanitizeForTerminal(brief));
              console.log('───────────────────────────────────────────');
              if (o.approve) return true;
              if (!process.stdin.isTTY) return false;
              const p = await import('@clack/prompts');
              const ok = await p.confirm({
                message: `Send it to ${profile.id}?`,
                initialValue: false,
              });
              return ok === true;
            },
            criterio: engine,
            runOptions: { timeoutMs: Number(o.timeout) * 60_000 },
            onEvent: printEvent,
            onStart: (h) => {
              cancel = () => h.cancel();
            },
          });
          process.removeListener('SIGINT', onSigint);
          console.log(`\n${report.run.state === 'completed' ? '✔' : '!'} ${formatRun(report.run)}`);
          if (report.run.error) console.log(`  error: ${clean(report.run.error, 300)}`);
          if (
            report.run.risk !== undefined &&
            engine.dispose(
              { value: report.run.risk, p: 0.8, why: [], source: 'rules' },
              'suggest_review',
              report.run.risk,
            ) === 'escalate'
          ) {
            console.log(
              '  ! high-risk diff: ask another profile to review it before accepting (cross review, M5).',
            );
          }
          const next =
            report.task.state === 'in_review'
              ? `review the changes on branch ${report.task.branch ?? '(read-only task)'}`
              : report.task.state === 'blocked'
                ? 'decide: wait for the limit to reset, or hand off to another profile (M4)'
                : `inspect with: hilvan task log ${short(taskId)}`;
          console.log(`  task is now ${report.task.state}; next: ${next}`);
          if (report.run.state !== 'completed') process.exitCode = 1;
        } catch (err) {
          fail(err);
        }
      },
    );

  task
    .command('recommend <id>')
    .description(
      "Which profiles may run this task, which are excluded and why, and Criterio's ranking.",
    )
    .option('--json', 'print as JSON')
    .action(async (ref: string, o: { json?: boolean }) => {
      try {
        const store = await openStore();
        const t = await store.loadTask(await store.resolveTaskId(ref));
        const { profiles } = await loadProfiles(profilesFile());
        const { engine } = await loadCriterio(criterioFile(userConfigDir()));
        const policy = await loadEffectivePolicy(defaultPolicyPaths(store.root));
        const rec = recommend(policy, t, profiles, engine, await store.listTasks());
        if (o.json) {
          const out = {
            taskType: rec.taskType,
            ranked: rec.ranked.map((r) => ({
              id: r.profile.id,
              score: r.score,
              why: r.why,
              requiresApproval: r.evaluation.requiresApproval,
            })),
            excluded: rec.excluded.map((x) => ({
              id: x.profile.id,
              exclusions: x.evaluation.exclusions,
            })),
          };
          return console.log(JSON.stringify(out, null, 2));
        }
        console.log(
          `task ${short(t.id)} · ${t.classification} · type ${rec.taskType}${t.requires.length ? ` · requires ${t.requires.join(', ')}` : ''}`,
        );
        if (!rec.ranked.length) console.log('✖ no eligible profile');
        rec.ranked.forEach((r, i) => {
          console.log(
            `${i === 0 ? '★' : ' '} ${r.profile.id} (${r.profile.app} @ ${describeDestination(r.profile)}) score ${r.score.toFixed(2)}${r.evaluation.requiresApproval ? ' · approval required' : ''}`,
          );
          for (const w of r.why) console.log(`     · ${w.signal}`);
        });
        for (const x of rec.excluded) {
          const reasons = x.evaluation.exclusions.map(
            (e) => `${e.reason}${e.enforced ? ' (enforced)' : ''}`,
          );
          console.log(`  ✖ ${x.profile.id}: ${reasons.join('; ')}`);
        }
        if (rec.ranked[0]) {
          console.log(
            `  next: hilvan task run ${short(t.id)} --profile ${rec.ranked[0].profile.id}  (you decide; nothing runs automatically)`,
          );
        }
      } catch (err) {
        fail(err);
      }
    });

  task
    .command('cancel <id>')
    .description('Ask the running agent of a task to stop.')
    .action(async (ref: string) => {
      try {
        const store = await openStore();
        const ok = await cancelTask(store, await store.resolveTaskId(ref));
        console.log(ok ? '✔ cancellation requested' : 'No active run for that task.');
      } catch (err) {
        fail(err);
      }
    });

  task
    .command('discard <id>')
    .description('Drop the task, its worktree and its branch without merging.')
    .option('-y, --yes', 'confirm')
    .action(async (ref: string, o: { yes?: boolean }) => {
      try {
        if (!o.yes) throw new Error('add --yes to discard the task and delete its branch');
        const store = await openStore();
        const t = await discardTask(store, await store.resolveTaskId(ref));
        console.log(`✔ task ${short(t.id)} discarded`);
      } catch (err) {
        fail(err);
      }
    });
}

function printTask(t: Task): void {
  console.log(`${t.id}  [${t.state}]  ${clean(t.title)}`);
  console.log(
    `  type ${t.taskType ?? '—'} · complexity ${t.complexity ?? '—'} · ${t.classification} · ${t.mode}` +
      (t.branch ? ` · branch ${t.branch}` : '') +
      (t.profileId ? ` · profile ${t.profileId}` : ''),
  );
  for (const r of t.runs) console.log(`  run ${short(r.id)}: ${formatRun(r)}`);
}
