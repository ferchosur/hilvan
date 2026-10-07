import os from 'node:os';
import path from 'node:path';

export const APP_DIR_NAME = 'hilvan';

export interface PlatformContext {
  platform: NodeJS.Platform;
  env: NodeJS.ProcessEnv;
  homedir: string;
}

export function currentPlatform(): PlatformContext {
  return { platform: process.platform, env: process.env, homedir: os.homedir() };
}

/** Path module matching the target platform, so Windows paths can be computed on Linux in tests. */
function pathFor(platform: NodeJS.Platform): path.PlatformPath {
  return platform === 'win32' ? path.win32 : path.posix;
}

/** Per-user configuration directory (ADR-0007). `HILVAN_CONFIG_DIR` overrides it. */
export function userConfigDir(ctx: PlatformContext = currentPlatform()): string {
  const p = pathFor(ctx.platform);
  if (ctx.env.HILVAN_CONFIG_DIR) return ctx.env.HILVAN_CONFIG_DIR;
  switch (ctx.platform) {
    case 'win32':
      return p.join(ctx.env.APPDATA ?? p.join(ctx.homedir, 'AppData', 'Roaming'), APP_DIR_NAME);
    case 'darwin':
      return p.join(ctx.homedir, 'Library', 'Application Support', APP_DIR_NAME);
    default:
      return p.join(ctx.env.XDG_CONFIG_HOME || p.join(ctx.homedir, '.config'), APP_DIR_NAME);
  }
}

/** Read-only managed policy whose `enforced` rules users cannot approve over (ADR-0009). */
export function managedPolicyFile(ctx: PlatformContext = currentPlatform()): string {
  const p = pathFor(ctx.platform);
  switch (ctx.platform) {
    case 'win32':
      return p.join(ctx.env.ProgramData ?? 'C:\\ProgramData', APP_DIR_NAME, 'policy.yaml');
    case 'darwin':
      return p.join('/Library/Application Support', APP_DIR_NAME, 'policy.yaml');
    default:
      return p.join('/etc', APP_DIR_NAME, 'policy.yaml');
  }
}

export function profilesFile(ctx: PlatformContext = currentPlatform()): string {
  return pathFor(ctx.platform).join(userConfigDir(ctx), 'profiles.yaml');
}

export function userPolicyFile(ctx: PlatformContext = currentPlatform()): string {
  return pathFor(ctx.platform).join(userConfigDir(ctx), 'policy.yaml');
}
