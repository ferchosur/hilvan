/**
 * SR-001/SR-002: neutralise terminal control sequences from untrusted text
 * (agent output, repository file names, task titles) before printing.
 */

// ESC-introduced sequences: CSI (ESC [ … final), OSC (ESC ] … BEL/ST), DCS/SOS/PM/APC (ESC P|X|^|_ … ST),
// and two-character escapes (ESC + one byte).
const ESC_SEQUENCES =
  /\u001b(?:\[[0-?]*[ -/]*[@-~]|\][^\u0007\u001b]*(?:\u0007|\u001b\\)?|[PX^_][^\u001b]*(?:\u001b\\)?|[@-Z\\-_])/g;

// 8-bit C1 equivalents: CSI (0x9b), OSC (0x9d), DCS (0x90), and the rest of C1.
const C1_SEQUENCES =
  /\u009b[0-?]*[ -/]*[@-~]|\u009d[^\u0007\u009c]*(?:\u0007|\u009c)?|[\u0080-\u009f]/g;

// C0 controls except \t and \n (and \r, handled separately).
const C0 = /[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/g;

// Bidirectional overrides/isolates and zero-width characters (Trojan Source, hidden text).
const INVISIBLE = /[\u200b-\u200f\u202a-\u202e\u2060-\u2064\u2066-\u2069\ufeff]/g;

export interface SanitizeOptions {
  /** Keep line breaks (default true). Titles and file names should pass false. */
  multiline?: boolean;
  /** Maximum length; longer text is cut with an ellipsis. */
  maxLength?: number;
}

export function sanitizeForTerminal(text: string, opts: SanitizeOptions = {}): string {
  let out = text
    .replace(ESC_SEQUENCES, '')
    .replace(C1_SEQUENCES, '')
    .replace(/\r\n?/g, '\n')
    .replace(C0, '')
    .replace(INVISIBLE, '');
  if (opts.multiline === false) out = out.replace(/[\n\t]+/g, ' ');
  if (opts.maxLength !== undefined && out.length > opts.maxLength) {
    out = `${out.slice(0, Math.max(0, opts.maxLength - 1))}…`;
  }
  return out;
}

/** True when the text contained anything that sanitising would remove. */
export function hasControlSequences(text: string): boolean {
  return sanitizeForTerminal(text) !== text.replace(/\r\n?/g, '\n');
}
