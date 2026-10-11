package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/store"
)

// clearScreen moves the cursor home and clears the terminal (ANSI, supported by Windows Terminal,
// WSL and most SSH clients). It is only printed to interactive terminals.
const clearScreen = "\x1b[H\x1b[2J"

// watchCommand is a read-only live panel of the project's tasks (ADR-0012). It never writes to
// .hilvan/: unlike openStore it does not mark interrupted runs as uncertain.
func watchCommand() *Command {
	return &Command{
		Name: "watch",
		Desc: "Live, read-only panel of this project's tasks. Ctrl+C to exit.",
		Options: []*Option{
			{Flags: "--once", Desc: "print one frame and exit (also when output is not a terminal)"},
			{Flags: "--interval <seconds>", Desc: "refresh period, 1 to 60", Default: "2"},
		},
		Run: func(c *Ctx) error {
			interval, err := strconv.Atoi(c.Str("interval"))
			if err != nil || interval < 1 || interval > 60 {
				return fmt.Errorf("--interval must be a whole number of seconds between 1 and 60")
			}
			once := c.Bool("once") || !c.App.Interactive
			s, err := store.Find(c.App.Cwd)
			if err != nil {
				return err
			}
			project, err := s.Project()
			if err != nil {
				return err
			}
			for {
				var frame string
				tasks, err := s.ListTasks()
				if err != nil {
					frame = "! cannot read tasks: " + clean(err.Error(), 200) + "\n"
				} else {
					frame = renderWatch(project.Name, tasks, s.ReadLock(), time.Now())
				}
				if c.App.Interactive && !once {
					fmt.Fprint(c.Out(), clearScreen)
				}
				fmt.Fprint(c.Out(), frame)
				if once {
					return nil
				}
				time.Sleep(time.Duration(interval) * time.Second)
			}
		},
	}
}

// renderWatch builds one frame. It is pure, so it can be tested without a terminal.
func renderWatch(project string, tasks []domain.Task, lock *store.Lock, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Hilvan · %s · %s\n\n", clean(project, 80), now.Format("15:04:05"))
	if lock != nil {
		fmt.Fprintf(&b, "Running: task %s, run %s since %s\n\n", short(lock.TaskID), short(lock.RunID), clean(lock.StartedAt, 40))
	}
	if len(tasks) == 0 {
		b.WriteString("No tasks yet. Create one with: hilvan task new \"<title>\"\n")
		return b.String()
	}
	fmt.Fprintf(&b, "%-12s %-9s %-16s %s\n", "STATE", "ID", "PROFILE", "TITLE")
	for _, t := range tasks {
		fmt.Fprintf(&b, "%-12s %-9s %-16s %s\n", t.State, short(t.ID), or(deref(t.ProfileID), "—"), clean(t.Title, 60))
		if n := len(t.Runs); n > 0 {
			fmt.Fprintf(&b, "  last run: %s\n", clean(FormatRun(t.Runs[n-1]), 160))
		}
	}
	return b.String()
}
