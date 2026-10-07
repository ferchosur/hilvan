import type { Command } from 'commander';
import { criterioFile, loadCriterio } from '../config/criterio-store.js';
import { userConfigDir } from '../config/paths.js';
import { usageByProfile } from '../core/stats.js';
import { ProjectStore } from '../store/project.js';

export function registerStatsCommand(program: Command): void {
  program
    .command('stats')
    .description(
      'Usage per profile in this project (as reported by the agent CLIs) and Criterio calibration.',
    )
    .option('--json', 'print as JSON')
    .action(async (o: { json?: boolean }) => {
      try {
        const store = await ProjectStore.find(process.cwd());
        const usage = usageByProfile(await store.listTasks());
        const { engine } = await loadCriterio(criterioFile(userConfigDir()));
        const crit = engine.stats();
        if (o.json) return console.log(JSON.stringify({ usage, criterio: crit }, null, 2));
        if (!usage.length) console.log('No runs yet in this project.');
        for (const u of usage) {
          const states = Object.entries(u.states)
            .map(([s, n]) => `${s} ${n}`)
            .join(', ');
          const cost = u.costRuns
            ? `~$${u.costUsd.toFixed(4)} estimated (${u.costRuns} runs)`
            : 'cost not reported';
          console.log(
            `${u.profileId}: ${u.runs} runs (${states}) · ${cost} · ${u.inputTokens} in / ${u.outputTokens} out tokens`,
          );
          if (u.lastLimits) {
            const w = u.lastLimits.windows
              .map((x) => `${x.name} ${Math.round(x.utilization * 100)} %`)
              .join(', ');
            console.log(
              `   last reported limits (${u.lastLimits.at.slice(0, 16).replace('T', ' ')}): ${w}`,
            );
          }
          if (u.accepted + u.discarded)
            console.log(`   outcomes: ${u.accepted} accepted, ${u.discarded} discarded`);
        }
        console.log(
          `criterio: ${crit.examples.taskType} type examples · brier ${crit.brier === null ? 'n/a' : crit.brier.toFixed(3)}`,
        );
      } catch (err) {
        console.error(`✖ ${(err as Error).message}`);
        process.exit(1);
      }
    });
}
