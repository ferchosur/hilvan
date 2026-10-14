package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ferchosur/hilvan/internal/core"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/security"
)

// chatSettings are the choices `hilvan chat` remembers between sessions (D-043).
type chatSettings struct {
	Profile     string `json:"profile,omitempty"`
	Interpreter string `json:"interpreter,omitempty"`
}

func chatFile(c *Ctx) string { return filepath.Join(c.App.Platform.UserConfigDir(), "chat.json") }

func loadChatSettings(c *Ctx) (chatSettings, error) {
	var st chatSettings
	data, err := os.ReadFile(chatFile(c))
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("chat settings are not valid JSON (%s): %v", chatFile(c), err)
	}
	return st, nil
}

func saveChatSettings(c *Ctx, st chatSettings) error {
	if err := os.MkdirAll(filepath.Dir(chatFile(c)), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return security.WriteFileAtomic(chatFile(c), append(data, '\n'), 0o600)
}

// chatCommand is the conversational entry point (spec 002). This first version handles the
// local slash commands only: nothing is sent to a model yet (FR-027).
func chatCommand() *Command {
	return &Command{
		Name: "chat",
		Desc: "Conversational mode. Slash commands: /profile, /interpreter, /help, /exit.",
		Run: func(c *Ctx) error {
			st, err := loadChatSettings(c)
			if err != nil {
				return err
			}
			c.Println("Hilvan chat. Type /help for commands, /exit to leave.")
			for {
				fmt.Fprint(c.Out(), "hilvan> ")
				line, ok := c.App.readLine()
				if !ok {
					c.Println()
					return nil
				}
				line = strings.TrimSpace(line)
				switch {
				case line == "":
				case line == "/exit":
					return nil
				case line == "/help":
					c.Println("  /profile [id]      show the profiles, or choose the one that runs tasks (remembered)")
					c.Println("  /interpreter [id]  show the profiles, or choose the one that reads your sentences (remembered)")
					c.Println("  /exit              leave")
				case line == "/profile" || strings.HasPrefix(line, "/profile "):
					st, err = chatChoose(c, st, "profile", strings.TrimSpace(strings.TrimPrefix(line, "/profile")))
					if err != nil {
						c.Eprintln("✖ " + err.Error())
					}
				case line == "/interpreter" || strings.HasPrefix(line, "/interpreter "):
					st, err = chatChoose(c, st, "interpreter", strings.TrimSpace(strings.TrimPrefix(line, "/interpreter")))
					if err != nil {
						c.Eprintln("✖ " + err.Error())
					}
				case strings.HasPrefix(line, "/"):
					c.Eprintln("✖ unknown command " + clean(line, 40) + " (try /help)")
				default:
					chatSentence(c, st, line)
				}
			}
		},
	}
}

// chatChoose lists the profiles, or chooses one for role ("profile" runs tasks, "interpreter" reads
// sentences) and remembers it. An unknown id is refused.
func chatChoose(c *Ctx, st chatSettings, role, id string) (chatSettings, error) {
	f, err := c.loadProfiles()
	if err != nil {
		return st, err
	}
	current := st.Profile
	if role == "interpreter" {
		current = st.Interpreter
	}
	if id == "" {
		if len(f.Profiles) == 0 {
			c.Println("  no profiles yet; add one with: hilvan profile add")
			return st, nil
		}
		shown := current
		if role == "interpreter" && shown == "" {
			shown = defaultInterpreter(f)
		}
		for _, p := range f.Profiles {
			mark := "  "
			if p.ID == shown {
				mark = "* "
			}
			c.Println(fmt.Sprintf("%s%s  %s @ %s", mark, p.ID, p.App, p.Destination.Launcher))
		}
		switch {
		case current != "":
		case role == "interpreter" && shown != "":
			c.Println("  (*) is the default interpreter until you choose one: /interpreter <id>")
		default:
			c.Println("  no " + role + " chosen yet: /" + role + " <id>")
		}
		return st, nil
	}
	p, err := core.FindProfile(f, id)
	if err != nil {
		var ids []string
		for _, x := range f.Profiles {
			ids = append(ids, x.ID)
		}
		return st, fmt.Errorf("%s; available: %s", err, strings.Join(ids, ", "))
	}
	if role == "interpreter" {
		st.Interpreter = p.ID
	} else {
		st.Profile = p.ID
	}
	if err := saveChatSettings(c, st); err != nil {
		return st, err
	}
	if role == "interpreter" {
		c.Println("✔ sentences will be read by " + p.ID + " (remembered for next time)")
	} else {
		c.Println("✔ tasks will run with " + p.ID + " (remembered for next time)")
	}
	return st, nil
}

// defaultInterpreter is the profile with routing "default" and the highest preference (spec 002,
// Q-014): the interpreter until the user chooses one.
func defaultInterpreter(f domain.ProfilesFile) string {
	best := ""
	bestPref := -1 << 30
	for _, p := range f.Profiles {
		if p.Routing == "default" && p.Preference > bestPref {
			best, bestPref = p.ID, p.Preference
		}
	}
	return best
}

// chatSentence interprets one sentence with the interpreter profile (spec 002, FR-022 to FR-024):
// the answer must be one operation of the closed list; the exact command is shown and runs only
// after confirmation.
func chatSentence(c *Ctx, st chatSettings, sentence string) {
	f, err := c.loadProfiles()
	if err != nil {
		c.Eprintln("✖ " + err.Error())
		return
	}
	id := st.Interpreter
	if id == "" {
		id = defaultInterpreter(f)
	}
	if id == "" {
		c.Println("  no interpreter yet: /interpreter <id>")
		return
	}
	p, err := core.FindProfile(f, id)
	if err != nil {
		c.Eprintln("✖ " + err.Error())
		return
	}
	dir := filepath.Join(c.App.Platform.UserConfigDir(), "interpret")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		c.Eprintln("✖ " + err.Error())
		return
	}
	c.Println("  asking " + p.ID + "…")
	plan, err := core.Interpret(p, sentence, c.App.Host, dir)
	if err != nil {
		c.Eprintln("✖ " + err.Error())
		return
	}
	argv, err := plan.Argv()
	if err != nil {
		c.Eprintln("✖ " + err.Error())
		return
	}
	c.Println("  proposed: hilvan " + strings.Join(quoteArgs(argv), " "))
	if !c.App.Confirm("Run it?", true) {
		c.Println("  cancelled; nothing was run")
		return
	}
	if code := Main(c.App, argv); code != 0 {
		c.Eprintln(fmt.Sprintf("✖ the command ended with code %d", code))
	}
}

// quoteArgs shows an argument with spaces in quotes, for display only.
func quoteArgs(argv []string) []string {
	out := make([]string, len(argv))
	for i, a := range argv {
		if strings.ContainsAny(a, " \t") {
			out[i] = `"` + a + `"`
		} else {
			out[i] = a
		}
	}
	return out
}
