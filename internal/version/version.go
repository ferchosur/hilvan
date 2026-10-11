// Package version holds the Hilvan version, injected at build time with
// -ldflags "-X github.com/ferchosur/hilvan/internal/version.Version=<v>".
package version

// Version is set at build time; 0.0.0 marks a development build.
var Version = "0.0.0"
