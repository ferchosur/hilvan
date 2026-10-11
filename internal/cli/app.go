package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ferchosur/hilvan/internal/config"
	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/launchers"
	"github.com/ferchosur/hilvan/internal/version"
)

// App holds the process environment a command runs in, so commands are testable.
type App struct {
	Stdout, Stderr io.Writer
	Stdin          io.Reader
	Cwd            string
	Platform       config.Platform
	Host           launchers.Host
	// Interactive is true when stdin and stdout are terminals.
	Interactive bool

	in *bufio.Reader
}

// NewApp describes the current process.
func NewApp() *App {
	cwd, _ := os.Getwd()
	return &App{
		Stdout: os.Stdout, Stderr: os.Stderr, Stdin: os.Stdin, Cwd: cwd,
		Platform: config.CurrentPlatform(), Host: core.CurrentHost(),
		Interactive: isTerminal(os.Stdin) && isTerminal(os.Stdout),
	}
}

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// Main runs the CLI and returns the process exit code.
func Main(app *App, args []string) int {
	if len(args) > 0 {
		allVersion := true
		for _, a := range args {
			allVersion = allVersion && (a == "-V" || a == "--version")
		}
		if allVersion {
			fmt.Fprintln(app.Stdout, version.Version)
			return 0
		}
	}
	root := Root()
	cmd, ctx, err := root.dispatch(args)
	var missing *missingCommand
	var usage *usageError
	switch {
	case errors.Is(err, errHelp):
		fmt.Fprint(app.Stdout, cmd.Help())
		return 0
	case errors.As(err, &missing):
		fmt.Fprint(app.Stderr, cmd.Help())
		return 1
	case errors.As(err, &usage):
		fmt.Fprintln(app.Stderr, usage.Error())
		return 1
	}
	ctx.App = app
	if err := cmd.Run(ctx); err != nil {
		fmt.Fprintln(app.Stderr, "✖ "+err.Error())
		return 1
	}
	return ctx.exit
}

// Root is the command tree.
func Root() *Command {
	root := &Command{Name: "hilvan", Desc: "Coordinate AI coding agents across providers and accounts."}
	return root.Add(
		doctorCommand(),
		profileCommand(),
		criterioCommand(),
		auditCommand(),
		policyCommand(),
		initCommand(),
		taskCommand(),
		handoffCommand(),
		reviewCommand(),
		acceptCommand(),
		statsCommand(),
		watchCommand(),
	)
}

// ── prompts: plain lines, so they work on Windows, WSL and over SSH (ADR-0012) ──

func (a *App) readLine() (string, bool) {
	if a.in == nil {
		a.in = bufio.NewReader(a.Stdin)
	}
	line, err := a.in.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return strings.TrimRight(line, "\r\n"), true
}

// Confirm asks a yes/no question; anything but y/yes/s/sí counts as the default.
func (a *App) Confirm(msg string, def bool) bool {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	fmt.Fprintf(a.Stdout, "? %s (%s) ", msg, hint)
	line, ok := a.readLine()
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes", "s", "si", "sí":
		return true
	case "n", "no":
		return false
	}
	return def
}

// Text asks for a value; validate returns a message when the value is not acceptable.
func (a *App) Text(msg, initial string, validate func(string) string) (string, bool) {
	for {
		if initial != "" {
			fmt.Fprintf(a.Stdout, "? %s [%s] ", msg, initial)
		} else {
			fmt.Fprintf(a.Stdout, "? %s ", msg)
		}
		line, ok := a.readLine()
		if !ok {
			return "", false
		}
		v := strings.TrimSpace(line)
		if v == "" {
			v = initial
		}
		if validate == nil {
			return v, true
		}
		if problem := validate(v); problem == "" {
			return v, true
		} else {
			fmt.Fprintln(a.Stdout, "  ✖ "+problem)
		}
	}
}

// Choice is an option of Select and MultiSelect.
type Choice struct{ Value, Label, Hint string }

func (a *App) listChoices(msg string, choices []Choice, selected map[string]bool) {
	fmt.Fprintln(a.Stdout, "? "+msg)
	for i, c := range choices {
		mark := " "
		if selected[c.Value] {
			mark = "*"
		}
		line := fmt.Sprintf("  %d) [%s] %s", i+1, mark, c.Label)
		if c.Hint != "" {
			line += "  (" + c.Hint + ")"
		}
		fmt.Fprintln(a.Stdout, line)
	}
}

// Select asks for one of the choices by number or value.
func (a *App) Select(msg string, choices []Choice) (string, bool) {
	a.listChoices(msg, choices, nil)
	for {
		fmt.Fprint(a.Stdout, "  number: ")
		line, ok := a.readLine()
		if !ok {
			return "", false
		}
		if v, ok := pick(strings.TrimSpace(line), choices); ok {
			return v, true
		}
	}
}

// MultiSelect asks for several choices as a comma list of numbers or values. An empty answer
// keeps the preselected ones; "-" selects none.
func (a *App) MultiSelect(msg string, choices []Choice, initial []string, required bool) ([]string, bool) {
	sel := map[string]bool{}
	for _, v := range initial {
		sel[v] = true
	}
	a.listChoices(msg+" (comma-separated numbers; Enter keeps the marked ones, - for none)", choices, sel)
	for {
		fmt.Fprint(a.Stdout, "  numbers: ")
		line, ok := a.readLine()
		if !ok {
			return nil, false
		}
		line = strings.TrimSpace(line)
		var out []string
		switch line {
		case "":
			out = append([]string{}, initial...)
		case "-":
			out = []string{}
		default:
			valid := true
			for _, part := range SplitList(line) {
				v, ok := pick(part, choices)
				if !ok {
					valid = false
					break
				}
				out = append(out, v)
			}
			if !valid {
				continue
			}
		}
		if required && len(out) == 0 {
			fmt.Fprintln(a.Stdout, "  ✖ choose at least one")
			continue
		}
		return out, true
	}
}

func pick(s string, choices []Choice) (string, bool) {
	for i, c := range choices {
		if s == fmt.Sprint(i+1) || s == c.Value {
			return c.Value, true
		}
	}
	return "", false
}
