// Package testutil holds helpers for the parity tests against TypeScript golden files (plan 002).
package testutil

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"testing"
)

// Generic round-trips v through JSON, so Go values compare with decoded golden values.
func Generic(t testing.TB, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Diff describes the first difference between got and want (decoded JSON values), or "".
// Numbers compare with a relative tolerance of 1e-9.
func Diff(path string, got, want any) string {
	switch w := want.(type) {
	case float64:
		g, ok := got.(float64)
		if !ok {
			return fmt.Sprintf("%s: got %v (%T), want %v", path, got, got, w)
		}
		if math.Abs(g-w) > 1e-9*math.Max(1, math.Max(math.Abs(g), math.Abs(w))) {
			return fmt.Sprintf("%s: got %v, want %v", path, g, w)
		}
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: got %v (%T), want object %v", path, got, got, w)
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			gv, gok := g[k]
			wv, wok := w[k]
			if gok != wok {
				return fmt.Sprintf("%s.%s: present in go=%v, in ts=%v (go %v, ts %v)", path, k, gok, wok, gv, wv)
			}
			if d := Diff(path+"."+k, gv, wv); d != "" {
				return d
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return fmt.Sprintf("%s: got %v, want %v", path, got, w)
		}
		for i := range w {
			if d := Diff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
	default:
		if got != want {
			return fmt.Sprintf("%s: got %#v, want %#v", path, got, want)
		}
	}
	return ""
}
