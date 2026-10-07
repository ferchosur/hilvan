import spawn from 'cross-spawn';

export interface RunResult {
  /** Exit code, or null if the process could not start or was killed. */
  code: number | null;
  stdout: string;
  stderr: string;
  /** Set when the executable is missing or the timeout expired. */
  error?: 'not_found' | 'timeout' | 'spawn_failed';
}

export interface RunOptions {
  timeoutMs?: number;
  env?: NodeJS.ProcessEnv;
  cwd?: string;
}

/**
 * Runs a short-lived command and collects its output.
 * Arguments are passed as an array without a shell; cross-spawn handles Windows
 * `.cmd` shims (npm-installed CLIs) with safe escaping.
 */
export function runCommand(
  command: string,
  args: readonly string[],
  opts: RunOptions = {},
): Promise<RunResult> {
  return new Promise((resolve) => {
    let stdout = '';
    let stderr = '';
    let settled = false;
    const finish = (r: RunResult) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      resolve(r);
    };

    const child = spawn(command, [...args], {
      env: opts.env ?? process.env,
      cwd: opts.cwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
    });
    const timer = setTimeout(() => {
      child.kill();
      finish({ code: null, stdout, stderr, error: 'timeout' });
    }, opts.timeoutMs ?? 15_000);

    child.stdout?.on('data', (d: Buffer) => {
      stdout += d.toString();
    });
    child.stderr?.on('data', (d: Buffer) => {
      stderr += d.toString();
    });
    child.on('error', (err: NodeJS.ErrnoException) => {
      finish({
        code: null,
        stdout,
        stderr,
        error: err.code === 'ENOENT' ? 'not_found' : 'spawn_failed',
      });
    });
    child.on('close', (code) => finish({ code, stdout, stderr }));
  });
}
