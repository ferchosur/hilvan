import type { Command } from 'commander';
import { profilesFile } from '../config/paths.js';
import { defaultDoctorDeps, formatResults, hasFailures, runDoctor } from './doctor.js';

export function registerDoctorCommand(program: Command): void {
  program
    .command('doctor')
    .description('Check the environment: Node, git, agent CLIs, WSL and profile configuration.')
    .option('--json', 'print results as JSON')
    .action(async (opts: { json?: boolean }) => {
      const results = await runDoctor(defaultDoctorDeps(profilesFile()));
      console.log(opts.json ? JSON.stringify(results, null, 2) : formatResults(results));
      if (hasFailures(results)) process.exitCode = 1;
    });
}
