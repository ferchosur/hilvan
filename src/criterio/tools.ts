import { tokenize } from './features.js';
import type { Reason } from './types.js';

/**
 * FR-021: words that reveal a brief needs a given tool. Built-in seeds (es/en) for common
 * enterprise tools; profiles add their own aliases, and Criterio learns from corrections.
 */
export const BUILTIN_TOOL_ALIASES: Readonly<Record<string, readonly string[]>> = {
  jira: [
    'jira',
    'ticket',
    'tickets',
    'incidencia',
    'incidencias',
    'issue',
    'issues',
    'sprint',
    'backlog',
    'epic',
    'historia de usuario',
    'user story',
  ],
  confluence: ['confluence', 'wiki', 'pagina de confluence', 'espacio de confluence'],
  slack: ['slack', 'canal de slack', 'slack channel'],
  teams: ['microsoft teams', 'teams', 'canal de teams'],
  github: ['github', 'pull request', 'github actions'],
  gitlab: ['gitlab', 'merge request'],
  linear: ['linear'],
  notion: ['notion'],
  sharepoint: ['sharepoint', 'onedrive'],
  outlook: [
    'outlook',
    'correo',
    'correos',
    'email',
    'calendario',
    'calendar',
    'reunion',
    'meeting',
  ],
  salesforce: ['salesforce', 'crm', 'oportunidades de venta'],
  servicenow: ['servicenow', 'change request', 'incidente de servicenow'],
  sap: ['sap'],
  datadog: ['datadog', 'monitor', 'monitoreo', 'metricas de produccion'],
  sentry: ['sentry', 'error tracking'],
  figma: ['figma', 'mockup', 'prototipo de diseno'],
  bigquery: ['bigquery'],
  snowflake: ['snowflake', 'data warehouse'],
};

/** Words in tool ids that say nothing about the tool. */
const GENERIC = new Set([
  'mcp',
  'claude',
  'ai',
  'api',
  'server',
  'servidor',
  'tool',
  'tools',
  'interno',
  'internal',
  'corp',
]);

export type ToolCatalog = Record<string, string[]>;

/** Catalog for the tools some profile provides: id words + built-in seeds + profile aliases. */
export function buildToolCatalog(
  profiles: ReadonlyArray<{
    tools: { provides: readonly string[]; aliases: Readonly<Record<string, readonly string[]>> };
  }>,
): ToolCatalog {
  const out: ToolCatalog = {};
  for (const p of profiles) {
    for (const id of p.tools.provides) {
      const words = id.split('-').filter((w) => w.length > 1 && !GENERIC.has(w));
      out[id] = [
        ...new Set([
          ...(out[id] ?? []),
          ...words,
          ...(BUILTIN_TOOL_ALIASES[id] ?? []),
          ...(p.tools.aliases[id] ?? []),
        ]),
      ];
    }
  }
  return out;
}

/** Aliases of `tool` found in the text, matching stemmed word sequences. */
export function aliasHits(text: string, aliases: readonly string[]): string[] {
  const toks = tokenize(text);
  const hits: string[] = [];
  for (const alias of aliases) {
    const a = tokenize(alias);
    if (!a.length) continue;
    for (let i = 0; i + a.length <= toks.length; i++) {
      if (a.every((w, j) => toks[i + j] === w)) {
        hits.push(alias);
        break;
      }
    }
  }
  return hits;
}

export interface ToolRule {
  p: number;
  why: Reason[];
}

export function toolRule(text: string, aliases: readonly string[]): ToolRule {
  const hits = aliasHits(text, aliases);
  if (!hits.length) return { p: 0.08, why: [] };
  return {
    p: Math.min(0.97, 0.75 + 0.1 * hits.length),
    why: hits.slice(0, 3).map((h) => ({ signal: `menciona "${h}"`, weight: 1 })),
  };
}
