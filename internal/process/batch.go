package process

import (
	"errors"
	"strings"
)

// BatchCommandLine builds the cmd.exe command line for a .cmd/.bat program (npm shims such as
// codex.cmd) so that no argument can inject commands (BatBadBut, CVE-2024-24576):
//   - every argument is wrapped in double quotes, which makes & | < > ( ) ^ literal for cmd.exe;
//   - backslashes before a closing quote are doubled, so the program (often node via %*) parses
//     the argument back exactly (MSVCRT rules);
//   - ", % and ! are rejected: cmd.exe expands or ends quoting on them even inside quotes.
//
// The result is `/d /s /c ""prog" "arg1" "arg2""` (/s strips the outer quotes, /d skips AutoRun).
// It is pure so it can be tested on any OS.
func BatchCommandLine(program string, args []string) (string, error) {
	var b strings.Builder
	b.WriteString(`/d /s /c "`)
	if err := quoteBatchArg(&b, program); err != nil {
		return "", err
	}
	for _, a := range args {
		b.WriteByte(' ')
		if err := quoteBatchArg(&b, a); err != nil {
			return "", err
		}
	}
	b.WriteByte('"')
	return b.String(), nil
}

func quoteBatchArg(b *strings.Builder, a string) error {
	if strings.ContainsAny(a, "\"%!\x00\r\n") {
		return ErrUnsafeBatchArgument
	}
	b.WriteByte('"')
	b.WriteString(a)
	// Trailing backslashes would escape the closing quote for the program's argv parser.
	for i := len(a) - 1; i >= 0 && a[i] == '\\'; i-- {
		b.WriteByte('\\')
	}
	b.WriteByte('"')
	return nil
}

// ErrUnsafeBatchArgument is returned for arguments that cmd.exe would expand even inside quotes.
var ErrUnsafeBatchArgument = errors.New(`argument contains a character that cmd.exe expands (" % !); not allowed for .cmd/.bat programs`)
