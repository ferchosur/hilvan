import { createHash } from 'node:crypto';
import path from 'node:path';
import { readTextSafe, writeFileAtomic } from '../security/fs.js';

/**
 * SR-012: `claude -p` runs hooks from the repository's `.claude/settings*.json` without a trust
 * dialog. Hilvan detects repository-provided agent configuration and asks the user to accept it
 * once per content hash before running.
 */

const CANDIDATES = ['.claude/settings.json', '.claude/settings.local.json', '.mcp.json'];
const MAX = 512 * 1024;

export interface RepoConfigItem {
  file: string;
  sha256: string;
  /** What makes it risky, for the warning. */
  risk: string;
}

export async function repoAgentConfig(root: string): Promise<RepoConfigItem[]> {
  const items: RepoConfigItem[] = [];
  for (const rel of CANDIDATES) {
    const text = await readTextSafe(path.join(root, rel), MAX).catch(() => null);
    if (text === null) continue;
    let risk: string | null = null;
    if (rel === '.mcp.json')
      risk = 'declares MCP servers (ignored by Hilvan via --strict-mcp-config)';
    else if (/"hooks"\s*:/.test(text))
      risk = 'defines hooks that run shell commands on agent events';
    if (risk)
      items.push({ file: rel, sha256: createHash('sha256').update(text).digest('hex'), risk });
  }
  return items;
}

function trustFile(stateDir: string) {
  return path.join(stateDir, 'trusted-config.json');
}

export async function untrusted(
  stateDir: string,
  items: readonly RepoConfigItem[],
): Promise<RepoConfigItem[]> {
  const text = await readTextSafe(trustFile(stateDir), MAX).catch(() => null);
  const trusted = new Set<string>(text ? (JSON.parse(text) as string[]) : []);
  // .mcp.json is neutralised by --strict-mcp-config; hooks are what need explicit trust.
  return items.filter((i) => i.file !== '.mcp.json' && !trusted.has(`${i.file}:${i.sha256}`));
}

export async function trust(stateDir: string, items: readonly RepoConfigItem[]): Promise<void> {
  const text = await readTextSafe(trustFile(stateDir), MAX).catch(() => null);
  const trusted = new Set<string>(text ? (JSON.parse(text) as string[]) : []);
  for (const i of items) trusted.add(`${i.file}:${i.sha256}`);
  await writeFileAtomic(trustFile(stateDir), JSON.stringify([...trusted].sort(), null, 2));
}
