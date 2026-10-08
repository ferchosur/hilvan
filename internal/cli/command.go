// Package cli is Hilvan's command-line interface. It has no third-party dependencies: a small
// command tree with the parsing rules, error messages and help layout the TypeScript version
// had with commander (ADR-0014).
package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// Option is a command option such as "-p, --profile <id>", "--json" or "--criteria <text>".
type Option struct {
	Flags    string
	Desc     string
	Default  string
	Collect  bool // repeatable: every occurrence is kept
	Required bool

	short, long, value string
}

func (o *Option) parse() {
	for _, part := range strings.Split(o.Flags, " ") {
		part = strings.TrimSuffix(part, ",")
		switch {
		case strings.HasPrefix(part, "--"):
			o.long = part[2:]
		case strings.HasPrefix(part, "-"):
			o.short = part[1:]
		case strings.HasPrefix(part, "<"):
			o.value = part
		}
	}
}

func (o *Option) takesValue() bool { return o.value != "" }

// Command is a node of the command tree.
type Command struct {
	Name    string
	Args    []string // "<id>" (required) or "[file]" (optional)
	Desc    string
	Options []*Option
	Subs    []*Command
	Run     func(*Ctx) error

	parent *Command
}

// Add appends subcommands.
func (c *Command) Add(subs ...*Command) *Command {
	for _, s := range subs {
		s.parent = c
		c.Subs = append(c.Subs, s)
	}
	return c
}

func (c *Command) path() string {
	if c.parent == nil {
		return c.Name
	}
	return c.parent.path() + " " + c.Name
}

func (c *Command) find(name string) *Command {
	for _, s := range c.Subs {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// Ctx is what an action receives.
type Ctx struct {
	App    *App
	Args   []string
	values map[string][]string
	exit   int
}

// Str returns an option's value (the last one) or its default.
func (c *Ctx) Str(long string) string {
	if v := c.values[long]; len(v) > 0 {
		return v[len(v)-1]
	}
	return ""
}

// Has reports whether an option was given (or has a default).
func (c *Ctx) Has(long string) bool { return len(c.values[long]) > 0 }

// Bool reports a flag.
func (c *Ctx) Bool(long string) bool { return c.Has(long) }

// All returns every value of a repeatable option.
func (c *Ctx) All(long string) []string { return c.values[long] }

// List splits an option's value on commas (like the TS `list` coercion); nil when absent.
func (c *Ctx) List(long string) []string {
	if !c.Has(long) {
		return nil
	}
	return SplitList(c.Str(long))
}

// SplitList splits "a, b,,c" into [a b c].
func SplitList(v string) []string {
	out := []string{}
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Fail marks the command as failed (exit code 1) without an error message.
func (c *Ctx) Fail() { c.exit = 1 }

// Out and Err are the command's output streams.
func (c *Ctx) Out() io.Writer { return c.App.Stdout }
func (c *Ctx) Err() io.Writer { return c.App.Stderr }

// Println writes a line to stdout.
func (c *Ctx) Println(a ...any) { fmt.Fprintln(c.App.Stdout, a...) }

// Printf writes formatted text to stdout.
func (c *Ctx) Printf(format string, a ...any) { fmt.Fprintf(c.App.Stdout, format, a...) }

// Eprintln writes a line to stderr.
func (c *Ctx) Eprintln(a ...any) { fmt.Fprintln(c.App.Stderr, a...) }

// usageError is a commander-style parsing error ("error: …").
type usageError struct{ msg string }

func (e *usageError) Error() string { return "error: " + e.msg }

var errHelp = errors.New("help")

func (c *Command) allOptions() []*Option {
	for _, o := range c.Options {
		o.parse()
	}
	return c.Options
}

func (c *Command) option(arg string) *Option {
	for _, o := range c.allOptions() {
		if (strings.HasPrefix(arg, "--") && arg[2:] == o.long) || (len(arg) == 2 && arg[1:] == o.short && o.short != "") {
			return o
		}
	}
	return nil
}

// dispatch finds the command for args and parses its options and arguments.
func (c *Command) dispatch(args []string) (*Command, *Ctx, error) {
	if len(c.Subs) > 0 {
		for i, a := range args {
			if a == "-h" || a == "--help" {
				if i == 0 {
					return c, nil, errHelp
				}
				continue
			}
			if strings.HasPrefix(a, "-") {
				continue
			}
			sub := c.find(a)
			if sub == nil && a == "help" {
				if i+1 < len(args) {
					if target := c.find(args[i+1]); target != nil {
						return target, nil, errHelp
					}
				}
				return c, nil, errHelp
			}
			if sub == nil {
				return c, nil, &usageError{fmt.Sprintf("unknown command '%s'", a)}
			}
			return sub.dispatch(append(append([]string{}, args[:i]...), args[i+1:]...))
		}
		return c, nil, &missingCommand{}
	}
	ctx := &Ctx{values: map[string][]string{}}
	for _, o := range c.allOptions() {
		if o.Default != "" {
			ctx.values[o.long] = []string{o.Default}
		}
	}
	explicit := map[string]bool{}
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if a == "-h" || a == "--help" {
			return c, nil, errHelp
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		name, inline, hasInline := strings.Cut(a, "=")
		o := c.option(name)
		if o == nil {
			return c, nil, &usageError{fmt.Sprintf("unknown option '%s'", a)}
		}
		if !o.takesValue() {
			ctx.values[o.long] = []string{"true"}
			continue
		}
		val := inline
		if !hasInline {
			if i+1 >= len(args) || (strings.HasPrefix(args[i+1], "-") && args[i+1] != "-") {
				return c, nil, &usageError{fmt.Sprintf("option '%s' argument missing", optionLabel(o))}
			}
			i++
			val = args[i]
		}
		if o.Collect {
			if !explicit[o.long] {
				ctx.values[o.long] = nil
			}
			ctx.values[o.long] = append(ctx.values[o.long], val)
		} else {
			ctx.values[o.long] = []string{val}
		}
		explicit[o.long] = true
	}
	for _, o := range c.allOptions() {
		if o.Required && !ctx.Has(o.long) {
			return c, nil, &usageError{fmt.Sprintf("required option '%s' not specified", optionLabel(o))}
		}
	}
	required := 0
	for _, a := range c.Args {
		if strings.HasPrefix(a, "<") {
			required++
		}
	}
	if len(positional) < required {
		return c, nil, &usageError{fmt.Sprintf("missing required argument '%s'", strings.Trim(c.Args[len(positional)], "<>[]"))}
	}
	if len(positional) > len(c.Args) {
		return c, nil, &usageError{fmt.Sprintf("too many arguments for '%s'. Expected %d argument%s but got %d: %s.",
			c.Name, len(c.Args), plural(len(c.Args)), len(positional), strings.Join(positional, ", "))}
	}
	ctx.Args = positional
	return c, ctx, nil
}

type missingCommand struct{}

func (*missingCommand) Error() string { return "missing command" }

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func optionLabel(o *Option) string { return o.Flags }

// ── help ──────────────────────────────────────────────────────────────────

const helpWidth = 80

func wrap(text string, indent, width int) string {
	words := strings.Fields(text)
	var lines []string
	line := ""
	for _, w := range words {
		if line != "" && len([]rune(line))+1+len([]rune(w)) > width-indent {
			lines = append(lines, line)
			line = w
		} else if line == "" {
			line = w
		} else {
			line += " " + w
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n"+strings.Repeat(" ", indent))
}

func (c *Command) usageLine() string {
	parts := []string{"Usage:", c.path(), "[options]"}
	if len(c.Subs) > 0 {
		parts = append(parts, "[command]")
	}
	parts = append(parts, c.Args...)
	return strings.Join(parts, " ")
}

func (c *Command) term() string {
	parts := []string{c.Name}
	if len(c.Options) > 0 {
		parts = append(parts, "[options]")
	}
	return strings.Join(append(parts, c.Args...), " ")
}

// Help renders the commander-style help of a command.
func (c *Command) Help() string {
	type row struct{ term, desc string }
	var opts []row
	for _, o := range c.allOptions() {
		desc := o.Desc
		if o.Default != "" {
			desc += fmt.Sprintf(" (default: %q)", o.Default)
		}
		opts = append(opts, row{o.Flags, desc})
	}
	if c.parent == nil {
		opts = append(opts, row{"-V, --version", "output the version number"})
	}
	opts = append(opts, row{"-h, --help", "display help for command"})
	var cmds []row
	for _, s := range c.Subs {
		cmds = append(cmds, row{s.term(), s.Desc})
	}
	if len(c.Subs) > 0 {
		cmds = append(cmds, row{"help [command]", "display help for command"})
	}
	width := 0
	for _, r := range append(append([]row{}, opts...), cmds...) {
		width = max(width, len([]rune(r.term)))
	}
	var b strings.Builder
	b.WriteString(c.usageLine() + "\n\n")
	if c.Desc != "" {
		b.WriteString(wrap(c.Desc, 0, helpWidth) + "\n\n")
	}
	section := func(title string, rows []row) {
		b.WriteString(title + ":\n")
		for _, r := range rows {
			pad := strings.Repeat(" ", width-len([]rune(r.term)))
			b.WriteString("  " + r.term + pad + "  " + wrap(r.desc, width+4, helpWidth) + "\n")
		}
	}
	section("Options", opts)
	if len(cmds) > 0 {
		b.WriteString("\n")
		section("Commands", cmds)
	}
	return b.String()
}
