import { randomBytes } from 'node:crypto';

const CROCKFORD = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

/** ULID: 48-bit time + 80-bit randomness, Crockford base32; sorts by creation time. */
export function ulid(now = Date.now(), rand: Uint8Array = randomBytes(10)): string {
  let time = '';
  let t = now;
  for (let i = 0; i < 10; i++) {
    time = (CROCKFORD[t % 32] ?? '0') + time;
    t = Math.floor(t / 32);
  }
  let bits = 0n;
  for (const b of rand) bits = (bits << 8n) | BigInt(b);
  let random = '';
  for (let i = 0; i < 16; i++) {
    random = (CROCKFORD[Number(bits & 31n)] ?? '0') + random;
    bits >>= 5n;
  }
  return time + random;
}

export const newTaskId = (): string => `tsk_${ulid()}`;
export const newRunId = (): string => `run_${ulid()}`;
