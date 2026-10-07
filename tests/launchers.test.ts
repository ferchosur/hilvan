import { describe, expect, it } from 'vitest';
import { buildInvocation, type HostContext, LauncherError } from '../src/launchers/index.js';

const cmd = {
  executable: 'claude',
  args: ['auth', 'status', '--json'],
  env: { CLAUDE_CONFIG_DIR: '/home/u/.hilvan-profiles/a' },
  unsetEnv: ['ANTHROPIC_API_KEY'],
};
const linux: HostContext = {
  platform: 'linux',
  insideWsl: false,
  env: { PATH: '/usr/bin', ANTHROPIC_API_KEY: 'stray', CLAUDE_CONFIG_DIR: '/other' },
};

describe('native launcher', () => {
  it('sets the profile config dir and removes stray provider keys', () => {
    const inv = buildInvocation({ launcher: 'native' }, cmd, linux);
    expect(inv.command).toBe('claude');
    expect(inv.args).toEqual(['auth', 'status', '--json']);
    expect(inv.env.CLAUDE_CONFIG_DIR).toBe('/home/u/.hilvan-profiles/a');
    expect(inv.env.ANTHROPIC_API_KEY).toBeUndefined();
    expect(inv.env.PATH).toBe('/usr/bin');
  });

  it('honours an executable override', () => {
    const inv = buildInvocation({ launcher: 'native', executable: '/opt/claude' }, cmd, linux);
    expect(inv.command).toBe('/opt/claude');
  });
});

describe('wsl launcher', () => {
  const win: HostContext = { platform: 'win32', insideWsl: false, env: { Path: 'C:\\x' } };

  it('runs inside the distro with env assignments and no shell', () => {
    const inv = buildInvocation(
      { launcher: 'wsl', distro: 'Ubuntu-24.04' },
      { ...cmd, cwd: '/home/u/repo' },
      win,
    );
    expect(inv.command).toBe('wsl.exe');
    expect(inv.args).toEqual([
      '-d',
      'Ubuntu-24.04',
      '--cd',
      '/home/u/repo',
      '--',
      'env',
      '-u',
      'ANTHROPIC_API_KEY',
      'CLAUDE_CONFIG_DIR=/home/u/.hilvan-profiles/a',
      'claude',
      'auth',
      'status',
      '--json',
    ]);
    expect(inv.env.CLAUDE_CONFIG_DIR).toBeUndefined();
  });

  it('is rejected when the host is not Windows', () => {
    expect(() => buildInvocation({ launcher: 'wsl', distro: 'U' }, cmd, linux)).toThrow(
      LauncherError,
    );
  });
});

describe('windows launcher', () => {
  it('forwards profile variables through WSLENV from inside WSL', () => {
    const host: HostContext = { platform: 'linux', insideWsl: true, env: { WSLENV: 'FOO' } };
    const inv = buildInvocation(
      { launcher: 'windows', executable: 'claude.exe' },
      { ...cmd, env: { CLAUDE_CONFIG_DIR: 'C:\\Users\\u\\.hilvan-profiles\\corp' } },
      host,
    );
    expect(inv.command).toBe('claude.exe');
    expect(inv.env.WSLENV).toBe('FOO:CLAUDE_CONFIG_DIR');
    expect(inv.env.CLAUDE_CONFIG_DIR).toBe('C:\\Users\\u\\.hilvan-profiles\\corp');
  });

  it('is rejected outside WSL', () => {
    expect(() => buildInvocation({ launcher: 'windows' }, cmd, linux)).toThrow(LauncherError);
  });
});
