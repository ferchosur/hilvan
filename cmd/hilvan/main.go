// Command hilvan coordinates AI coding agents across providers and accounts (ADR-0014).
package main

import (
	"os"

	"github.com/ferchosur/hilvan/internal/cli"
)

func main() {
	os.Exit(cli.Main(cli.NewApp(), os.Args[1:]))
}
