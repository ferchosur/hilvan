package adapters

import (
	"bytes"
	"encoding/json"
	"math"

	"github.com/ferchosur/hilvan/internal/domain"
)

// Parsers from each CLI's JSONL to normalised events. Pure; tested with real fixtures and with
// golden cases from the TypeScript parsers.

type jsonObj = map[string]any

func obj(v any) jsonObj {
	m, _ := v.(map[string]any)
	return m
}

// strOf returns the value when it is a string (like `typeof v === 'string'`).
func strOf(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func numOf(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok && !math.IsInf(f, 0) && !math.IsNaN(f)
}

// truthy is JavaScript truthiness for decoded JSON values.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	}
	return true
}

func parseObject(line string) jsonObj {
	var v any
	if json.Unmarshal([]byte(line), &v) != nil {
		return nil
	}
	return obj(v)
}

// firstString is `str(a) ?? str(b) ?? …`: the first value that is a string, even "".
func firstString(vs ...any) string {
	for _, v := range vs {
		if s, ok := strOf(v); ok {
			return s
		}
	}
	return ""
}

func ptr(f float64) *float64 { return &f }

// orderedWindows reads rate_limit_info.unifiedWindows keeping the document's key order
// (Object.entries order), which the TypeScript parser preserves.
func orderedWindows(line string) []domain.LimitWindow {
	var doc struct {
		Info struct {
			Windows json.RawMessage `json:"unifiedWindows"`
		} `json:"rate_limit_info"`
	}
	if json.Unmarshal([]byte(line), &doc) != nil || len(doc.Info.Windows) == 0 {
		return []domain.LimitWindow{}
	}
	dec := json.NewDecoder(bytes.NewReader(doc.Info.Windows))
	out := []domain.LimitWindow{}
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return out
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return out
		}
		name, _ := kt.(string)
		var w any
		if dec.Decode(&w) != nil {
			return out
		}
		u, ok := numOf(obj(w)["utilization"])
		if !ok {
			continue
		}
		lw := domain.LimitWindow{Name: name, Utilization: math.Min(1, math.Max(0, u))}
		if r, ok := numOf(obj(w)["resetsAt"]); ok {
			lw.ResetsAt = ptr(r)
		}
		out = append(out, lw)
	}
	return out
}

// ParseClaudeLine normalises one line of `claude -p --output-format stream-json`.
func ParseClaudeLine(line string) []Event {
	e := parseObject(line)
	if e == nil {
		return []Event{{Type: EvRaw, Line: line}}
	}
	switch e["type"] {
	case "system":
		switch e["subtype"] {
		case "init":
			servers := []string{}
			if list, ok := e["mcp_servers"].([]any); ok {
				for _, s := range list {
					if name, _ := strOf(obj(s)["name"]); name != "" {
						servers = append(servers, name)
					}
				}
			}
			sid, _ := strOf(e["session_id"])
			model, _ := strOf(e["model"])
			return []Event{{Type: EvSession, SessionID: str(sid), Model: model, MCPServers: servers}}
		case "api_retry":
			attempt, _ := numOf(e["attempt"])
			msg, ok := strOf(e["error"])
			if !ok {
				msg = "unknown"
			}
			return []Event{{Type: EvRetry, Attempt: int(attempt), Error: msg}}
		}
		return nil
	case "assistant":
		content, ok := obj(e["message"])["content"].([]any)
		if !ok {
			return nil
		}
		var out []Event
		for _, raw := range content {
			c := obj(raw)
			if c == nil {
				continue
			}
			if c["type"] == "text" {
				if text, _ := strOf(c["text"]); text != "" {
					out = append(out, Event{Type: EvMessage, Text: text})
				}
			}
			if c["type"] == "tool_use" {
				in := obj(c["input"])
				name, ok := strOf(c["name"])
				if !ok {
					name = "tool"
				}
				out = append(out, Event{Type: EvTool, Name: name, Status: "started", Summary: firstString(in["command"], in["file_path"], in["pattern"])})
			}
		}
		return out
	case "user":
		content, ok := obj(e["message"])["content"].([]any)
		if !ok {
			return nil
		}
		var out []Event
		for _, raw := range content {
			c := obj(raw)
			if c == nil || c["type"] != "tool_result" {
				continue
			}
			status := "completed"
			if truthy(c["is_error"]) {
				status = "failed"
			}
			out = append(out, Event{Type: EvTool, Name: "result", Status: status})
		}
		return out
	case "rate_limit_event":
		status, ok := strOf(obj(e["rate_limit_info"])["status"])
		if !ok {
			status = "unknown"
		}
		return []Event{{Type: EvLimits, Status: status, Windows: orderedWindows(line)}}
	case "result":
		usage := obj(e["usage"])
		denials := 0
		if list, ok := e["permission_denials"].([]any); ok {
			denials = len(list)
		}
		isError := e["is_error"] == true
		kind := "error"
		if s, ok := strOf(e["subtype"]); ok {
			kind = s
		}
		if status, ok := numOf(e["api_error_status"]); ok {
			switch status {
			case 429:
				kind = "rate_limit"
			case 401, 403:
				kind = "authentication_failed"
			case 402:
				kind = "billing_error"
			}
		}
		u := Event{Type: EvUsage}
		if cost, ok := numOf(e["total_cost_usd"]); ok {
			u.CostUSD = ptr(cost)
		}
		if usage != nil {
			in := 0.0
			for _, k := range []string{"input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens"} {
				if v, ok := numOf(usage[k]); ok {
					in += v
				}
			}
			outTok, _ := numOf(usage["output_tokens"])
			u.InputTokens, u.OutputTokens = ptr(in), ptr(outTok)
		}
		r := Event{Type: EvResult, OK: !isError, Text: firstString(e["result"]), PermissionDenials: denials}
		if sid, _ := strOf(e["session_id"]); sid != "" {
			r.SessionID = str(sid)
		}
		r.StopReason, _ = strOf(e["stop_reason"])
		if isError {
			r.Error, r.HasError = kind, true
		}
		return []Event{u, r}
	}
	return nil
}

// CodexParser normalises `codex exec --json`. It keeps the last agent message to use as the
// result text of turn.completed, so each run needs its own parser.
type CodexParser struct{ lastMessage string }

// Parse normalises one line.
func (p *CodexParser) Parse(line string) []Event {
	e := parseObject(line)
	if e == nil {
		return []Event{{Type: EvRaw, Line: line}}
	}
	switch e["type"] {
	case "thread.started":
		id, _ := strOf(e["thread_id"])
		return []Event{{Type: EvSession, SessionID: str(id)}}
	case "item.started", "item.completed":
		item := obj(e["item"])
		kind, _ := strOf(item["type"])
		switch kind {
		case "agent_message":
			if e["type"] == "item.completed" {
				p.lastMessage, _ = strOf(item["text"])
				return []Event{{Type: EvMessage, Text: p.lastMessage}}
			}
			return nil
		case "command_execution", "file_change", "mcp_tool_call", "web_search":
			status := "completed"
			if e["type"] == "item.started" {
				status = "started"
			} else if item["status"] == "failed" {
				status = "failed"
			}
			return []Event{{Type: EvTool, Name: kind, Status: status, Summary: firstString(item["command"], item["query"], item["tool"])}}
		}
		return nil
	case "turn.completed":
		u := obj(e["usage"])
		in, _ := numOf(u["input_tokens"])
		out, _ := numOf(u["output_tokens"])
		reasoning, _ := numOf(u["reasoning_output_tokens"])
		return []Event{
			{Type: EvUsage, InputTokens: ptr(in), OutputTokens: ptr(out + reasoning)},
			{Type: EvResult, OK: true, Text: p.lastMessage},
		}
	case "turn.failed":
		msg, ok := strOf(obj(e["error"])["message"])
		if !ok {
			msg = "turn failed"
		}
		return []Event{{Type: EvResult, OK: false, Text: p.lastMessage, Error: msg, HasError: true}}
	case "error":
		msg, ok := strOf(e["message"])
		if !ok {
			msg = "error"
		}
		return []Event{{Type: EvRetry, Attempt: 0, Error: msg}}
	}
	return nil
}
