// Package security holds Hilvan's defensive helpers (docs/security/requisitos.md).
package security

import (
	"errors"
	"strings"
)

// ErrUnsafeArgument is returned for arguments that could inject through Windows .cmd shims (SR-004).
var ErrUnsafeArgument = errors.New("argument contains a newline or NUL byte; pass text via stdin")

// AssertSafeArgs requires plain single-line arguments; prompts travel via stdin instead.
func AssertSafeArgs(args []string) error {
	for _, a := range args {
		if strings.ContainsAny(a, "\x00\r\n") {
			return ErrUnsafeArgument
		}
	}
	return nil
}
