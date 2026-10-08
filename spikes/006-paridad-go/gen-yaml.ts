// Spike 006: YAML escrito por la librería `yaml` de npm (la que usa Hilvan) con valores conflictivos.
import { writeFileSync } from 'node:fs';
import { stringify } from 'yaml';
const value = {
  version: 1,
  profiles: [{ id: 'claude-corp', app: 'claude-code', label: 'Corp: "Claude" #1', configDir: null,
    accepts: ['corporate', 'confidential'], tags: ['yes', 'no', 'on', 'off', 'null', '~', '0700', '1e3', '2026-10-07'],
    multiline: 'línea 1\nlínea 2\n', aliases: { jira: ['ticket', 'incidencia'] }, preference: -1.5, empty: '' }],
};
writeFileSync(new URL('testdata/profiles.yaml', import.meta.url), stringify(value));
writeFileSync(new URL('testdata/profiles.json', import.meta.url), JSON.stringify(value));
