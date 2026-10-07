#!/usr/bin/env node
import { readFileSync } from 'node:fs';
import { Command } from 'commander';
import { profilesFile } from '../config/paths.js';
import { defaultDoctorDeps, formatResults, hasFailures, runDoctor } from './doctor.js';

// Same relative location from src/cli (tsx) and dist/cli (build).
const pkg = JSON.parse(readFileSync(new URL('../../package.json', import.meta.url), 'utf8')) as {
  version: string;
};

const program = new Command()
  .name('ao')
  .description('Coordinate AI coding agents across providers and accounts.')
  .version(pkg.version);

program
  .command('doctor')
  .description('Check the environment: Node, git, agent CLIs, WSL and profile configuration.')
  .option('--json', 'print results as JSON')
  .action(async (opts: { json?: boolean }) => {
    const results = await runDoctor(defaultDoctorDeps(profilesFile()));
    console.log(opts.json ? JSON.stringify(results, null, 2) : formatResults(results));
    if (hasFailures(results)) process.exitCode = 1;
  });

await program.parseAsync();
