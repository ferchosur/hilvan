package security

import (
	"regexp"
	"strings"
	"unicode/utf16"
)

// SR-001/SR-002: neutralise terminal control sequences from untrusted text
// (agent output, repository file names, task titles) before printing.
var (
	// ESC-introduced sequences: CSI, OSC (… BEL/ST), DCS/SOS/PM/APC (… ST) and two-byte escapes.
	escSequences = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)?|[PX^_][^\x1b]*(?:\x1b\\)?|[@-Z\\-_])`)
	// 8-bit C1 equivalents: CSI (0x9b), OSC (0x9d) and the rest of C1.
	c1Sequences = regexp.MustCompile(`\x{9b}[0-?]*[ -/]*[@-~]|\x{9d}[^\x07\x{9c}]*(?:\x07|\x{9c})?|[\x{80}-\x{9f}]`)
	crlf        = regexp.MustCompile(`\r\n?`)
	// C0 controls except \t and \n (\r is normalised before).
	c0 = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
	// Bidirectional overrides/isolates and zero-width characters (Trojan Source, hidden text).
	invisible  = regexp.MustCompile(`[\x{200b}-\x{200f}\x{202a}-\x{202e}\x{2060}-\x{2064}\x{2066}-\x{2069}\x{feff}]`)
	lineBreaks = regexp.MustCompile(`[\n\t]+`)
)

// SanitizeOptions controls SanitizeForTerminal.
type SanitizeOptions struct {
	// SingleLine collapses line breaks and tabs; titles and file names should set it.
	SingleLine bool
	// MaxLength cuts longer text with an ellipsis, counted in UTF-16 units as in the TS version.
	// Zero means no limit.
	MaxLength int
}

// SanitizeForTerminal removes escape sequences, C0/C1 controls and invisible characters.
func SanitizeForTerminal(text string, opts SanitizeOptions) string {
	out := escSequences.ReplaceAllString(text, "")
	out = c1Sequences.ReplaceAllString(out, "")
	out = crlf.ReplaceAllString(out, "\n")
	out = c0.ReplaceAllString(out, "")
	out = invisible.ReplaceAllString(out, "")
	if opts.SingleLine {
		out = lineBreaks.ReplaceAllString(out, " ")
	}
	if opts.MaxLength > 0 {
		if u := utf16.Encode([]rune(out)); len(u) > opts.MaxLength {
			out = string(utf16.Decode(u[:opts.MaxLength-1])) + "…"
		}
	}
	return out
}

// HasControlSequences reports whether sanitising would remove anything.
func HasControlSequences(text string) bool {
	return SanitizeForTerminal(text, SanitizeOptions{}) != crlf.ReplaceAllString(text, "\n")
}

// Line is a convenience for single-line, length-bounded output.
func Line(text string, max int) string {
	return strings.TrimSpace(SanitizeForTerminal(text, SanitizeOptions{SingleLine: true, MaxLength: max}))
}
