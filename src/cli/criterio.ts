import { rm } from 'node:fs/promises';
import type { Command } from 'commander';
import { criterioFile, loadCriterio, saveCriterio } from '../config/criterio-store.js';
import { userConfigDir } from '../config/paths.js';
import { type Decision, TASK_TYPES, type TaskType } from '../criterio/index.js';

function fmt(d: Decision<unknown>, label: string, disp?: string): string {
  const v = d.value === null ? '—' : String(d.value);
  const head = `${label}: ${v}  (p=${d.p.toFixed(2)}, ${d.source}${disp ? `, ${disp}` : ''})`;
  const why = d.why
    .slice(0, 4)
    .map((r) => `    · ${r.signal} (${r.weight >= 0 ? '+' : ''}${r.weight.toFixed(2)})`);
  return [head, ...why].join('\n');
}

export function registerCriterioCommands(program: Command): void {
  const cmd = program
    .command('criterio')
    .description(
      'Local decision engine: typed suggestions with confidence and reasons (no network).',
    );

  cmd
    .command('ask <title>')
    .description('Ask Criterio about a task brief: type, complexity and data sensitivity.')
    .option('--text <text>', 'brief body')
    .option('--criteria <n>', 'number of acceptance criteria')
    .option('--classification <c>', 'current classification', 'personal')
    .option('--json', 'print as JSON')
    .action(
      async (
        title: string,
        o: { text?: string; criteria?: string; classification: string; json?: boolean },
      ) => {
        const { engine, warning } = await loadCriterio(criterioFile(userConfigDir()));
        if (warning) console.error(`! ${warning}`);
        const s = {
          title,
          ...(o.text ? { text: o.text } : {}),
          ...(o.criteria ? { criteriaCount: Number(o.criteria) } : {}),
        };
        const type = engine.taskType(s);
        const complexity = engine.complexity(s, type.value);
        const cls = engine.suggestClassification(s, o.classification);
        const out = {
          taskType: { ...type, disposition: engine.dispose(type, 'label') },
          complexity: { ...complexity, disposition: engine.dispose(complexity, 'label') },
          classification: { ...cls, disposition: engine.dispose(cls, 'harden_classification') },
        };
        if (o.json) return console.log(JSON.stringify(out, null, 2));
        console.log(fmt(type, 'type', out.taskType.disposition));
        console.log(fmt(complexity, 'complexity', out.complexity.disposition));
        console.log(
          fmt(
            { ...cls, value: cls.value ? `harden to ${cls.value}` : `keep ${o.classification}` },
            'classification',
            out.classification.disposition,
          ),
        );
      },
    );

  cmd
    .command('teach <title>')
    .description(
      'Teach Criterio the right task type for a brief (corrections improve later suggestions).',
    )
    .requiredOption('--type <type>', TASK_TYPES.join(' | '))
    .option('--text <text>', 'brief body')
    .action(async (title: string, o: { type: string; text?: string }) => {
      if (!(TASK_TYPES as readonly string[]).includes(o.type)) {
        console.error(`✖ --type must be one of: ${TASK_TYPES.join(', ')}`);
        process.exit(1);
      }
      const file = criterioFile(userConfigDir());
      const { engine } = await loadCriterio(file);
      engine.learnTaskType({ title, ...(o.text ? { text: o.text } : {}) }, o.type as TaskType);
      await saveCriterio(file, engine);
      console.log(`✔ learned: "${title}" → ${o.type}`);
    });

  cmd
    .command('stats')
    .description('Show how much Criterio has learned and how well calibrated it is.')
    .option('--json', 'print as JSON')
    .action(async (o: { json?: boolean }) => {
      const { engine } = await loadCriterio(criterioFile(userConfigDir()));
      const s = engine.stats();
      if (o.json) return console.log(JSON.stringify(s, null, 2));
      console.log(
        `examples: type ${s.examples.taskType}, sensitivity ${s.examples.sensitivity}, acceptance ${s.examples.acceptance}`,
      );
      console.log(
        `brier score: ${s.brier === null ? 'n/a (no outcomes yet)' : s.brier.toFixed(3)} (0 = perfect, 0.25 = coin flip)`,
      );
      for (const b of s.reliability.filter((x) => x.count)) {
        console.log(
          `  p ${b.from.toFixed(1)}–${b.to.toFixed(1)}: predicted ${b.meanP.toFixed(2)}, observed ${b.observed.toFixed(2)} (n=${b.count})`,
        );
      }
      for (const p of s.profiles)
        console.log(`  ${p.key}: ${(p.mean * 100).toFixed(0)} % success (n=${p.n})`);
    });

  cmd
    .command('reset')
    .description('Forget everything Criterio learned (rules remain).')
    .option('-y, --yes', 'confirm')
    .action(async (o: { yes?: boolean }) => {
      if (!o.yes) {
        console.error('✖ add --yes to delete the learned model');
        process.exit(1);
      }
      await rm(criterioFile(userConfigDir()), { force: true });
      console.log('✔ Criterio model deleted; rules only from now on');
    });
}
