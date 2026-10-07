import path from 'node:path';
import type { Command } from 'commander';
import { verifyAudit } from '../store/audit.js';

export function registerAuditCommands(program: Command): void {
  const audit = program.command('audit').description('Inspect the project audit log.');
  audit
    .command('verify [file]')
    .description('Verify the hash chain of the audit log (default: .hilvan/events.jsonl).')
    .action(async (file?: string) => {
      const target = file ?? path.join(process.cwd(), '.hilvan', 'events.jsonl');
      const r = await verifyAudit(target);
      if (r.ok) {
        console.log(`✔ audit log intact (${r.events} events) — ${target}`);
      } else {
        console.error(`✖ audit log broken at line ${r.brokenAt}: ${r.reason} — ${target}`);
        process.exitCode = 1;
      }
    });
}
