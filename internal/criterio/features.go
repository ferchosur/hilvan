// Package criterio is Hilvan's local decision engine (ADR-0011, FR-020): typed decisions with
// confidence and reasons, learned from the user's corrections. No I/O, no network.
//
// Feature hashing and tokenisation must match the TypeScript implementation exactly, or
// models learned there would stop meaning anything (verified in spike 006).
package criterio

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf16"

	"golang.org/x/text/unicode/norm"
)

// Feature is a sparse feature: a readable name (kept in memory only, for explanations) and a value.
type Feature struct {
	Name  string
	Value float64
}

// Buckets is the number of hash buckets. Models store weights per bucket, never the original text.
const Buckets = 1 << 16

// FNV1a is the 32-bit FNV-1a hash over UTF-16 code units (like the TS charCodeAt loop).
func FNV1a(s string) uint32 {
	h := uint32(0x811c9dc5)
	for _, u := range utf16.Encode([]rune(s)) {
		h ^= uint32(u)
		h *= 0x01000193
	}
	return h
}

// Bucket maps a feature name to its weight row.
func Bucket(name string) uint32 { return FNV1a(name) % Buckets }

var stopwords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields("a al algo and are as at be by con de del el en es esta este for from ha in is it la las lo los " +
		"of on or para por que se sin su the to un una y que we you our nuestro") {
		m[w] = true
	}
	return m
}()

const stemLen = 6

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Tokenize lowercases, strips diacritics, splits on non-alphanumerics, drops stopwords and stems.
func Tokenize(text string) []string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(text)) {
		if !unicode.Is(unicode.M, r) {
			b.WriteRune(r)
		}
	}
	out := []string{}
	for _, raw := range nonAlnum.Split(b.String(), -1) {
		if len(raw) < 2 || stopwords[raw] {
			continue
		}
		out = append(out, Stem(raw))
	}
	return out
}

// Stem is a cheap language-agnostic stemmer: refactorizar/refactor → refact.
func Stem(word string) string {
	if len(word) > stemLen {
		return word[:stemLen]
	}
	return word
}

var (
	sensitivePath = regexp.MustCompile(`(?i)(^|/)(\.env|.*secret.*|.*credential.*|.*\.pem|.*\.key|id_rsa.*|\.github/workflows/.*|.*migrations?/.*|.*auth.*|.*security.*|package\.json|package-lock\.json|pnpm-lock\.yaml|yarn\.lock|go\.(mod|sum)|cargo\.(toml|lock)|requirements.*\.txt|dockerfile|.*\.tf)$`)
	testPath      = regexp.MustCompile(`(?i)(^|/)(tests?|__tests__|spec)/|\.(test|spec)\.[a-z0-9]+$`)
)

// IsSensitivePath reports paths whose changes deserve extra care (secrets, CI, auth, lockfiles…).
func IsSensitivePath(p string) bool {
	return sensitivePath.MatchString(strings.ReplaceAll(p, `\`, "/"))
}

// IsTestPath reports test files and directories.
func IsTestPath(p string) bool { return testPath.MatchString(strings.ReplaceAll(p, `\`, "/")) }

func bin(name string, v float64, edges []float64) Feature {
	i := 0
	for i < len(edges) && v > edges[i] {
		i++
	}
	return Feature{Name: name + ":b" + itoa(i), Value: 1}
}

// DiffFeatures turns diff facts into features.
func DiffFeatures(d DiffSignals) []Feature {
	lines := d.LinesAdded + d.LinesDeleted
	sensitive, tests := 0, 0
	for _, p := range d.Paths {
		if IsSensitivePath(p) {
			sensitive++
		}
		if IsTestPath(p) {
			tests++
		}
	}
	delRatio := 0.0
	if lines != 0 {
		delRatio = float64(d.LinesDeleted) / float64(lines)
	}
	feats := []Feature{
		bin("diff.files", float64(d.FilesChanged), []float64{1, 3, 10, 30}),
		bin("diff.lines", float64(lines), []float64{20, 100, 400, 1500}),
		{Name: "diff.del_ratio", Value: delRatio},
	}
	if sensitive > 0 {
		feats = append(feats, Feature{Name: "diff.sensitive", Value: float64(min(sensitive, 5)) / 5})
	}
	if tests > 0 {
		feats = append(feats, Feature{Name: "diff.tests", Value: 1})
	}
	if tests == 0 && d.FilesChanged > 0 {
		feats = append(feats, Feature{Name: "diff.no_tests", Value: 1})
	}
	return feats
}

// TaskFeatures is a bag of stemmed words and bigrams plus diff signals. Duplicates count once.
// No bias or length features: class priors come from the rules.
func TaskFeatures(s TaskSignals) []Feature {
	toks := Tokenize(s.Title + " " + s.Title + " " + s.Text)
	seen := map[string]bool{}
	var feats []Feature
	for i, tok := range toks {
		w := "w:" + tok
		if !seen[w] {
			seen[w] = true
			feats = append(feats, Feature{Name: w, Value: 1})
		}
		if i+1 < len(toks) {
			bg := "b:" + tok + "_" + toks[i+1]
			if !seen[bg] {
				seen[bg] = true
				feats = append(feats, Feature{Name: bg, Value: 0.5})
			}
		}
	}
	if s.CriteriaCount != nil {
		feats = append(feats, bin("criteria", float64(*s.CriteriaCount), []float64{1, 3, 6}))
	}
	if s.Diff != nil {
		feats = append(feats, DiffFeatures(*s.Diff)...)
	}
	return feats
}

// unique keeps the first occurrence of each string, like [...new Set(xs)].
func unique(xs []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
