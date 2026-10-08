// Package domain holds Hilvan's data model (docs/design/modelo-de-datos.md).
// It performs no I/O. Validation mirrors the zod schemas of the TypeScript
// implementation (plan 002): same rules, same limits, same message wording.
package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Issue is one validation problem at a path such as profiles[0].destination.distro.
type Issue struct {
	Path    string
	Message string
}

// Issues is a list of validation problems; nil means valid.
type Issues []Issue

// Pretty formats issues like zod's prettifyError.
func (is Issues) Pretty() string {
	lines := make([]string, 0, 2*len(is))
	for _, i := range is {
		lines = append(lines, "✖ "+i.Message)
		if i.Path != "" {
			lines = append(lines, "  → at "+i.Path)
		}
	}
	return strings.Join(lines, "\n")
}

func (is Issues) Error() string { return is.Pretty() }

// Err returns nil when there are no issues, so callers can write `return v.Validate().Err()`.
func (is Issues) Err() error {
	if len(is) == 0 {
		return nil
	}
	return is
}

type issues struct{ list Issues }

func (b *issues) add(path, format string, args ...any) {
	b.list = append(b.list, Issue{Path: path, Message: fmt.Sprintf(format, args...)})
}

func field(base, name string) string {
	if base == "" {
		return name
	}
	return base + "." + name
}

func index(base string, i int) string { return base + "[" + strconv.Itoa(i) + "]" }

// IDPattern is the stable identifier for profiles, tags, tools and rule ids.
var IDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

const idMessage = "must be lowercase letters, digits or dashes (max 63)"

func (b *issues) id(path, v string) {
	if !IDPattern.MatchString(v) {
		b.add(path, idMessage)
	}
}

func (b *issues) ids(path string, vs []string) {
	for i, v := range vs {
		b.id(index(path, i), v)
	}
}

func (b *issues) nonEmpty(path, v string) {
	if v == "" {
		b.add(path, "Too small: expected string to have >=1 characters")
	}
}

func (b *issues) nonEmptyAll(path string, vs []string) {
	for i, v := range vs {
		b.nonEmpty(index(path, i), v)
	}
}

func (b *issues) oneOf(path, v string, options ...string) {
	for _, o := range options {
		if v == o {
			return
		}
	}
	quoted := make([]string, len(options))
	for i, o := range options {
		quoted[i] = strconv.Quote(o)
	}
	b.add(path, "Invalid option: expected one of %s", strings.Join(quoted, "|"))
}

func (b *issues) pattern(path, v string, re *regexp.Regexp) {
	if !re.MatchString(v) {
		b.add(path, "Invalid string: must match pattern /%s/", re.String())
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
