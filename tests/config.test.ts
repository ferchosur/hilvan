import { describe, expect, it } from 'vitest';
import {
  managedPolicyFile,
  type PlatformContext,
  profilesFile,
  userConfigDir,
} from '../src/config/paths.js';
import { ConfigError, parseProfiles } from '../src/config/profiles.js';

const linux: PlatformContext = { platform: 'linux', env: {}, homedir: '/home/u' };
const win: PlatformContext = {
  platform: 'win32',
  env: { APPDATA: 'C:\\Users\\u\\AppData\\Roaming', ProgramData: 'C:\\ProgramData' },
  homedir: 'C:\\Users\\u',
};
const mac: PlatformContext = { platform: 'darwin', env: {}, homedir: '/Users/u' };

describe('config paths', () => {
  it('uses XDG on Linux', () => {
    expect(userConfigDir(linux)).toBe('/home/u/.config/hilvan');
    expect(userConfigDir({ ...linux, env: { XDG_CONFIG_HOME: '/x' } })).toBe('/x/hilvan');
  });

  it('uses APPDATA on Windows', () => {
    expect(profilesFile(win)).toBe('C:\\Users\\u\\AppData\\Roaming\\hilvan\\profiles.yaml');
  });

  it('uses Application Support on macOS', () => {
    expect(userConfigDir(mac)).toBe('/Users/u/Library/Application Support/hilvan');
  });

  it('honours HILVAN_CONFIG_DIR', () => {
    expect(userConfigDir({ ...linux, env: { HILVAN_CONFIG_DIR: '/tmp/hv' } })).toBe('/tmp/hv');
  });

  it('places the managed policy in a system location', () => {
    expect(managedPolicyFile(linux)).toBe('/etc/hilvan/policy.yaml');
    expect(managedPolicyFile(win)).toBe('C:\\ProgramData\\hilvan\\policy.yaml');
  });
});

describe('parseProfiles', () => {
  it('treats an empty file as no profiles', () => {
    expect(parseProfiles('').profiles).toEqual([]);
  });

  it('parses a two-environment setup (AC-015)', () => {
    const yaml = `
version: 1
profiles:
  - id: claude-corp
    label: Claude corporate (Windows)
    app: claude-code
    auth: { mode: subscription }
    configDir: 'C:\\Users\\u\\.hilvan-profiles\\claude-corp'
    destination: { launcher: native }
    accepts: [corporate, confidential]
    tags: [corporate]
  - id: claude-personal
    label: Claude personal (WSL)
    app: claude-code
    auth: { mode: subscription }
    configDir: /home/u/.hilvan-profiles/claude-personal
    destination: { launcher: wsl, distro: Ubuntu-24.04 }
    accepts: [public, personal]
    tags: [personal]
`;
    const file = parseProfiles(yaml);
    expect(file.profiles.map((p) => p.destination.launcher)).toEqual(['native', 'wsl']);
  });

  it('reports readable errors', () => {
    expect(() => parseProfiles('version: 2')).toThrow(ConfigError);
    expect(() => parseProfiles('version: [')).toThrow(ConfigError);
  });
});
