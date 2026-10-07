/**
 * SR-004: arguments must be plain single-line values. Newlines and NUL bytes enable
 * injection through Windows `.cmd` shims and confuse logs; prompts travel via stdin instead.
 */
export class UnsafeArgumentError extends Error {
  override name = 'UnsafeArgumentError';
}

const FORBIDDEN = /[\u0000\r\n]/;

export function assertSafeArgs(args: readonly string[]): void {
  for (const a of args) {
    if (FORBIDDEN.test(a)) {
      throw new UnsafeArgumentError('argument contains a newline or NUL byte; pass text via stdin');
    }
  }
}
