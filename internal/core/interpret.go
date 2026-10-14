package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ferchosur/hilvan/internal/adapters"
	"github.com/ferchosur/hilvan/internal/domain"
	"github.com/ferchosur/hilvan/internal/launchers"
)

// Operations are the only operations a sentence may become (spec 002, FR-022, D-038). Anything else
// is refused: the interpreter proposes, Hilvan decides what can run.
var Operations = []string{"task new", "task list", "task show", "task log", "task recommend", "task run", "review", "accept", "stats"}

// interpretTimeout bounds one interpretation (D-040: one short call, no session).
const interpretTimeout = 2 * time.Minute

// Plan is an interpreted sentence: an operation and its arguments, not yet run.
type Plan struct {
	Operation string
	Args      map[string]string
}

// interpretArgs are the arguments an operation may take.
var interpretArgs = []string{"title", "objective", "criteria", "mode", "task", "profile"}

// InterpretSchema is the JSON schema the interpreter must answer with (A-010 verified on Linux).
func InterpretSchema() string {
	ops := append(append([]string{}, Operations...), "none")
	enc := func(v any) string { b, _ := json.Marshal(v); return string(b) }
	props := map[string]any{
		"operation": map[string]any{"type": "string", "enum": ops},
		"title":     map[string]any{"type": "string"},
		"objective": map[string]any{"type": "string"},
		"criteria":  map[string]any{"type": "string"},
		"mode":      map[string]any{"type": "string", "enum": []string{"read", "write"}},
		"task":      map[string]any{"type": "string"},
		"profile":   map[string]any{"type": "string"},
	}
	return enc(map[string]any{"type": "object", "properties": props, "required": []string{"operation"}, "additionalProperties": false})
}

// interpretPrompt asks for one operation. The sentence is the only user text; no history (NFR-016).
func interpretPrompt(sentence string) string {
	return "Convierte la frase del usuario en una operación de Hilvan. Responde solo con el JSON pedido. " +
		"Operaciones: " + strings.Join(Operations, ", ") + ". Si la frase no encaja, responde operation \"none\". " +
		"Frase: " + sentence
}

// Interpret asks the interpreter profile for an operation and validates the answer against the closed
// list. dir is a private directory for the CLI's output files.
func Interpret(p *domain.Profile, sentence string, host launchers.Host, dir string) (Plan, error) {
	if !contains(p.Accepts, "personal") {
		return Plan{}, fmt.Errorf("the interpreter %s does not accept personal sentences", p.ID)
	}
	adapter, ok := adapters.For(p.App)
	if !ok {
		return Plan{}, fmt.Errorf("no adapter for %s", p.App)
	}
	cmd, outFile := adapter.InterpretCommand(p, host.Env, InterpretSchema(), dir)
	inv, err := launchers.BuildInvocation(p.Destination, cmd, host)
	if err != nil {
		return Plan{}, err
	}
	stdout, err := adapters.RunOnce(inv, interpretPrompt(sentence), interpretTimeout)
	if err != nil {
		return Plan{}, err
	}
	answer, err := adapter.ParseInterpretation(stdout, outFile)
	if err != nil {
		return Plan{}, err
	}
	return ValidatePlan(answer)
}

// ValidatePlan turns the interpreter's JSON answer into a plan, refusing anything outside the list.
func ValidatePlan(answer string) (Plan, error) {
	var raw map[string]string
	if err := json.Unmarshal([]byte(answer), &raw); err != nil {
		return Plan{}, fmt.Errorf("the interpreter answer is not a valid object")
	}
	op := raw["operation"]
	if op == "none" || op == "" {
		return Plan{}, fmt.Errorf("the sentence does not match any operation Hilvan knows")
	}
	if !contains(Operations, op) {
		return Plan{}, fmt.Errorf("operation %q is not allowed", op)
	}
	args := map[string]string{}
	for _, k := range interpretArgs {
		v := strings.TrimSpace(raw[k])
		if v == "" {
			continue
		}
		if len(v) > 2000 {
			return Plan{}, fmt.Errorf("argument %s is too long", k)
		}
		if (k == "task" || k == "profile") && strings.HasPrefix(v, "-") {
			return Plan{}, fmt.Errorf("argument %s may not start with '-'", k)
		}
		args[k] = v
	}
	if args["mode"] != "" && args["mode"] != "read" && args["mode"] != "write" {
		return Plan{}, fmt.Errorf("mode must be read or write")
	}
	return Plan{Operation: op, Args: args}, nil
}

// Argv is the Hilvan command line for the plan, one argument per value, with no shell.
func (pl Plan) Argv() ([]string, error) {
	a := pl.Args
	need := func(k string) (string, error) {
		if a[k] == "" {
			return "", fmt.Errorf("%s needs a %s", pl.Operation, k)
		}
		return a[k], nil
	}
	switch pl.Operation {
	case "task new":
		title, err := need("title")
		if err != nil {
			return nil, err
		}
		mode := a["mode"]
		if mode == "" {
			mode = "read"
		}
		argv := []string{"task", "new", title, "--mode", mode}
		if a["objective"] != "" {
			argv = append(argv, "--objective", a["objective"])
		}
		if a["criteria"] != "" {
			argv = append(argv, "--criteria", a["criteria"])
		}
		return argv, nil
	case "task list", "stats":
		return strings.Fields(pl.Operation), nil
	case "task show", "task log", "task recommend", "task run", "review", "accept":
		task, err := need("task")
		if err != nil {
			return nil, err
		}
		argv := append(strings.Fields(pl.Operation), task)
		if pl.Operation == "task run" && a["profile"] != "" {
			argv = append(argv, "--profile", a["profile"])
		}
		return argv, nil
	}
	return nil, fmt.Errorf("operation %q is not allowed", pl.Operation)
}
