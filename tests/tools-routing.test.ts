import { describe, expect, it } from 'vitest';
import { adapterFor } from '../src/adapters/index.js';
import { parseAliases } from '../src/cli/profile.js';
import { discoverTools, toolIdFromName } from '../src/core/profiles.js';
import { recommend } from '../src/core/recommend.js';
import { buildToolCatalog, Criterio } from '../src/criterio/index.js';
import { ProfileSchema, type Task } from '../src/domain/index.js';
import { mergePolicies } from '../src/policy/evaluate.js';

const corp = ProfileSchema.parse({
  id: 'claude-corp',
  label: 'corp',
  app: 'claude-code',
  auth: { mode: 'subscription' },
  configDir: '/h/corp',
  destination: { launcher: 'native' },
  accepts: ['public', 'personal', 'corporate'],
  tags: ['corporate'],
  routing: 'reserved',
  tools: {
    provides: ['jira', 'intranet-search'],
    aliases: { 'intranet-search': ['portal de empleados'] },
  },
});
const personal = ProfileSchema.parse({
  id: 'claude-personal',
  label: 'personal',
  app: 'claude-code',
  auth: { mode: 'subscription' },
  configDir: '/h/personal',
  destination: { launcher: 'native' },
  accepts: ['public', 'personal', 'corporate'],
});
const task = (over: Partial<Task>) =>
  ({
    id: 'tsk_01JABCDEFGHJKMNPQRSTVWXYZ0',
    title: 'x',
    classification: 'personal',
    requires: [],
    runs: [],
    ...over,
  }) as Task;
const rec = (t: Task, profiles = [corp, personal]) =>
  recommend(mergePolicies({}), t, profiles, new Criterio(), [], 1);

describe('reserved routing (FR-021)', () => {
  it('sends everything that does not need its tools to the default profile', () => {
    const r = rec(task({ title: 'Refactorizar el parser' }));
    expect(r.ranked.map((x) => x.profile.id)).toEqual(['claude-personal']);
    const ex = r.excluded.find((x) => x.profile.id === 'claude-corp');
    expect(ex?.evaluation.exclusions[0]).toMatchObject({ source: 'routing' });
    expect(ex?.evaluation.exclusions[0]?.reason).toMatch(
      /reserved for tasks that need jira, intranet-search/,
    );
  });

  it('uses the reserved profile when the task requires one of its tools', () => {
    const r = rec(task({ requires: ['jira'] }));
    expect(r.ranked.map((x) => x.profile.id)).toEqual(['claude-corp']);
    expect(r.ranked[0]?.why[0]?.signal).toMatch(/reservado: la tarea requiere jira/);
  });

  it('keeps the reserved profile as a last resort when nothing else is eligible', () => {
    const r = rec(task({ classification: 'corporate' }), [
      corp,
      { ...personal, accepts: ['public'] },
    ]);
    expect(r.ranked.map((x) => x.profile.id)).toEqual(['claude-corp']);
    expect(r.ranked[0]?.why[0]?.signal).toMatch(/único perfil elegible/);
  });
});

describe('required tools inferred by Criterio (FR-021)', () => {
  const catalog = buildToolCatalog([corp, personal]);

  it('builds a catalog from provides, id words, built-in and profile aliases', () => {
    expect(catalog.jira).toEqual(expect.arrayContaining(['jira', 'ticket', 'incidencia']));
    expect(catalog['intranet-search']).toEqual(
      expect.arrayContaining(['intranet', 'search', 'portal de empleados']),
    );
  });

  it.each([
    ['Revisar las incidencias del sprint en Jira', ['jira']],
    ['Buscar en la intranet la política de vacaciones', ['intranet-search']],
    ['Consultar el portal de empleados', ['intranet-search']],
    ['Refactorizar el parser de eventos', []],
  ])('%s → %j', (title, expected) => {
    expect(new Criterio().requiredTools({ title }, catalog).value).toEqual(expected);
  });

  it('learns from confirmations (new words) and corrections', () => {
    const c = new Criterio();
    const s = { title: 'Actualizar el estado del caso de soporte 4521' };
    expect(c.requiredTools(s, catalog).value).toEqual([]);
    for (const n of ['4521', '3310', '9087', '1200', '5555', '7001', '8123', '2222']) {
      c.learnRequiredTools({ title: `Actualizar el estado del caso de soporte ${n}` }, catalog, [
        'jira',
      ]);
    }
    expect(c.requiredTools({ title: 'Cerrar el caso de soporte 6060' }, catalog).value).toContain(
      'jira',
    );
    expect(c.stats().examples.tools).toBeGreaterThan(0);
  });
});

describe('tool discovery', () => {
  it('parses `claude mcp list` without keeping targets and maps names to ids', () => {
    const out = [
      'Checking MCP server health…',
      '',
      'claude.ai Jira Cloud: https://mcp.example/jira?token=SECRET - ✔ Connected',
      'intranet: node /opt/intranet.js - ✗ Failed to connect',
      'repo-x: npx foo - ⏸ Pending approval',
    ].join('\n');
    const tools = adapterFor('claude-code').parseToolList(out);
    expect(tools).toEqual([
      { name: 'claude.ai Jira Cloud', status: 'connected' },
      { name: 'intranet', status: 'failed' },
      { name: 'repo-x', status: 'pending' },
    ]);
    expect(JSON.stringify(tools)).not.toContain('SECRET');
    expect(toolIdFromName('claude.ai Jira Cloud')).toBe('claude-ai-jira-cloud');
    expect(toolIdFromName('Búsqueda Intranet')).toBe('busqueda-intranet');
  });

  it('parses `codex mcp list --json`', () => {
    expect(adapterFor('codex').parseToolList('[{"name":"docs","enabled":true}]')).toEqual([
      { name: 'docs', status: 'unknown' },
    ]);
    expect(adapterFor('codex').parseToolList('not json')).toEqual([]);
  });

  it('compares discovered servers with provides', async () => {
    const d = await discoverTools(corp, {
      host: { platform: 'linux', insideWsl: false, env: {} },
      run: async (inv) => {
        expect(inv.args).toEqual(['mcp', 'list']);
        expect(inv.cwd).toBeDefined(); // run outside the repository
        return {
          code: 0,
          stdout: 'jira: https://x - ✔ Connected\nconfluence: https://y - ✔ Connected',
          stderr: '',
        };
      },
      configDirIssues: async () => ({ issues: [], warnings: [] }),
    });
    expect(d.missing).toEqual(['confluence']);
    expect(d.stale).toEqual(['intranet-search']);
  });

  it('parses --alias options', () => {
    expect(parseAliases(['jira=ticket, incidencia', 'jira=epic', 'wiki=confluence'])).toEqual({
      jira: ['ticket', 'incidencia', 'epic'],
      wiki: ['confluence'],
    });
    expect(() => parseAliases(['jira'])).toThrow(/tool=word1,word2/);
  });
});
