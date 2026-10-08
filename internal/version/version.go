// Package version holds the Hilvan version, injected at build time with
// -ldflags "-X hilvan/internal/version.Version=<v>".
package version

// Version matches package.json until the TypeScript implementation is retired (plan 002, G5).
var Version = "0.0.0"
