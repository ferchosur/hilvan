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
	"github.com/ferchosur/hilvan/internal/security"
)

// chatSettings are the choices `hilvan chat` remembers between sessions (D-043).
type chatSettings struct {
	Profile string `json:"profile,omitempty"`
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
		Desc: "Conversational mode. Slash commands: /profile, /help, /exit.",
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
					c.Println("  /profile [id]  show the profiles, or choose the one that runs tasks (remembered)")
					c.Println("  /exit          leave")
				case line == "/profile" || strings.HasPrefix(line, "/profile "):
					st, err = chatProfile(c, st, strings.TrimSpace(strings.TrimPrefix(line, "/profile")))
					if err != nil {
						c.Eprintln("✖ " + err.Error())
					}
				case strings.HasPrefix(line, "/"):
					c.Eprintln("✖ unknown command " + clean(line, 40) + " (try /help)")
				default:
					c.Println("  natural-language requests are not available yet; use /profile, /help or /exit.")
				}
			}
		},
	}
}

// chatProfile lists the profiles, or chooses one and remembers it. An unknown id is refused.
func chatProfile(c *Ctx, st chatSettings, id string) (chatSettings, error) {
	f, err := c.loadProfiles()
	if err != nil {
		return st, err
	}
	if id == "" {
		if len(f.Profiles) == 0 {
			c.Println("  no profiles yet; add one with: hilvan profile add")
			return st, nil
		}
		for _, p := range f.Profiles {
			mark := "  "
			if p.ID == st.Profile {
				mark = "* "
			}
			c.Println(fmt.Sprintf("%s%s  %s @ %s", mark, p.ID, p.App, p.Destination.Launcher))
		}
		if st.Profile == "" {
			c.Println("  no profile chosen yet: /profile <id>")
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
	st.Profile = p.ID
	if err := saveChatSettings(c, st); err != nil {
		return st, err
	}
	c.Println("✔ tasks will run with " + p.ID + " (remembered for next time)")
	return st, nil
}
