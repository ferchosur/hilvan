// Package store persists a project's state in its .hilvan/ folder (ADR-0007): project, tasks,
// briefs, run logs, hand-offs, reviews, the run lock and the audit log.
package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ferchosur/hilvan/internal/jsfmt"
	"github.com/ferchosur/hilvan/internal/security"
)

// Genesis is the prev hash of the first event.
var Genesis = strings.Repeat("0", 64)

// AuditEvent is one entry of the hash-chained audit log (SR-017/SR-018). Data holds hashes, ids
// and paths only — never package contents or secrets — and must not contain nil values.
type AuditEvent struct {
	TS        string         `json:"ts"`
	Type      string         `json:"type"`
	TaskID    string         `json:"taskId,omitempty"`
	RunID     string         `json:"runId,omitempty"`
	ProfileID string         `json:"profileId,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// ChainedEvent is an event as stored: the event plus prev and hash.
type ChainedEvent struct {
	AuditEvent
	Prev string `json:"prev"`
	Hash string `json:"hash"`
}

func toGeneric(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(raw, &m)
}

// ChainHash is sha256(prev + canonical(event)), with sorted keys, exactly as the TypeScript
// implementation computes it, so Go verifies logs written by TS and vice versa.
func ChainHash(prev string, event map[string]any) string {
	h := sha256.New()
	h.Write([]byte(prev))
	h.Write([]byte(jsfmt.Canonical(event)))
	return hex.EncodeToString(h.Sum(nil))
}

// lastHash reads the hash of the last line without loading the whole file.
func lastHash(file string) (string, error) {
	f, err := os.Open(file)
	if errors.Is(err, fs.ErrNotExist) {
		return Genesis, nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if st.Size() == 0 {
		return Genesis, nil
	}
	n := min(st.Size(), 64*1024)
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, st.Size()-n); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	lines := strings.Split(strings.TrimRight(string(buf), "\n\r\t "), "\n")
	var last struct {
		Hash *string `json:"hash"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil || last.Hash == nil {
		return "", errors.New("audit log tail is malformed")
	}
	return *last.Hash, nil
}

// AppendAudit appends an event to the chain. One writer per project (guarded by the run lock).
func AppendAudit(file string, ev AuditEvent) (ChainedEvent, error) {
	if err := security.EnsurePrivateDir(filepath.Dir(file)); err != nil {
		return ChainedEvent{}, err
	}
	prev, err := lastHash(file)
	if err != nil {
		return ChainedEvent{}, err
	}
	generic, err := toGeneric(ev)
	if err != nil {
		return ChainedEvent{}, err
	}
	chained := ChainedEvent{AuditEvent: ev, Prev: prev, Hash: ChainHash(prev, generic)}
	var line bytes.Buffer
	enc := json.NewEncoder(&line)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(chained); err != nil {
		return ChainedEvent{}, err
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return ChainedEvent{}, err
	}
	defer f.Close()
	_, err = f.Write(line.Bytes())
	return chained, err
}

// VerifyResult is the outcome of checking the chain.
type VerifyResult struct {
	OK     bool   `json:"ok"`
	Events int    `json:"events"`
	Broken int    `json:"brokenAt,omitempty"` // 1-based line of the first problem
	Reason string `json:"reason,omitempty"`
}

// VerifyAudit checks every link and every hash. A missing log is an empty, valid chain.
func VerifyAudit(file string, maxBytes int64) (VerifyResult, error) {
	if maxBytes == 0 {
		maxBytes = 512 * 1024 * 1024
	}
	text, found, err := security.ReadTextSafe(file, maxBytes)
	if err != nil {
		return VerifyResult{}, err
	}
	if !found {
		return VerifyResult{OK: true}, nil
	}
	prev := Genesis
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	for i, l := range lines {
		var ev map[string]any
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			return VerifyResult{Events: i, Broken: i + 1, Reason: "invalid JSON"}, nil
		}
		p, _ := ev["prev"].(string)
		hash, _ := ev["hash"].(string)
		delete(ev, "prev")
		delete(ev, "hash")
		if p != prev {
			return VerifyResult{Events: i, Broken: i + 1, Reason: "chain link mismatch"}, nil
		}
		if ChainHash(prev, ev) != hash {
			return VerifyResult{Events: i, Broken: i + 1, Reason: "content was modified"}, nil
		}
		prev = hash
	}
	return VerifyResult{OK: true, Events: len(lines)}, nil
}
