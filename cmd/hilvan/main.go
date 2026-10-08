// Command hilvan coordinates AI coding agents across providers and accounts.
//
// Go implementation in progress (ADR-0014, plan 002). Until G4 only --version
// and help are available; the TypeScript CLI remains the reference.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ferchosur/hilvan/internal/version"
)

const usage = `Usage: hilvan [options] [command]

Coordinate AI coding agents across providers and accounts.

Options:
  -V, --version             output the version number
  -h, --help                display help for command
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}
	switch args[0] {
	case "-V", "--version":
		fmt.Fprintln(stdout, version.Version)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	}
	fmt.Fprintf(stderr, "error: command %q is not available in the Go build yet (plan 002)\n", args[0])
	return 1
}
