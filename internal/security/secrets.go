package security

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// SecretMatch is a detected secret at byte offsets [Start, End).
type SecretMatch struct {
	Kind       string
	Start, End int
}

// jsSpace is JavaScript's \s, which also matches \v and Unicode spaces (Go's \s does not).
const jsSpace = `\t\n\x0b\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

func jsRegexp(pattern string) *regexp.Regexp {
	p := strings.NewReplacer(`[^\s`, `[^`+jsSpace, `[\s\S]`, `[\s\S]`, `\s`, `[`+jsSpace+`]`).Replace(pattern)
	return regexp.MustCompile(p)
}

// SR-013: known token formats plus a high-entropy fallback. No network, no dependencies.
var secretPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"private-key", jsRegexp(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`)},
	{"anthropic-key", jsRegexp(`\bsk-ant-[A-Za-z0-9_-]{20,}`)},
	{"openai-key", jsRegexp(`\bsk-(?:proj-|svcacct-|admin-)?[A-Za-z0-9_-]{20,}`)},
	{"github-token", jsRegexp(`\b(?:gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{40,})\b`)},
	{"aws-access-key", jsRegexp(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)},
	{"google-api-key", jsRegexp(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{"slack-token", jsRegexp(`\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{"stripe-key", jsRegexp(`\b[rs]k_(?:live|test)_[A-Za-z0-9]{20,}`)},
	{"jwt", jsRegexp(`\beyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	{"url-credentials", jsRegexp(`(?i)\b[a-z][a-z0-9+.-]*://[^\s:/@]+:[^\s@/]{3,}@[^\s]+`)},
	{"assignment", jsRegexp(`(?i)\b(?:password|passwd|pwd|secret|api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret)\b\s*[:=]\s*["']?([^\s"']{8,})`)},
}

var (
	candidate = regexp.MustCompile(`[A-Za-z0-9+/_=-]{32,}`)
	lower     = regexp.MustCompile(`[a-z]`)
	upper     = regexp.MustCompile(`[A-Z]`)
	digit     = regexp.MustCompile(`\d`)
)

// Entropy is the Shannon entropy in bits per character.
func Entropy(s string) float64 {
	if s == "" {
		return 0
	}
	counts := map[rune]int{}
	for _, r := range s {
		counts[r]++
	}
	n := float64(utf16Len(s))
	h := 0.0
	for _, c := range counts {
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}

func overlaps(m SecretMatch, list []SecretMatch) bool {
	for _, x := range list {
		if m.Start < x.End && x.Start < m.End {
			return true
		}
	}
	return false
}

// FindSecrets returns the secrets in text, ordered by position.
func FindSecrets(text string) []SecretMatch {
	var found []SecretMatch
	for _, p := range secretPatterns {
		for _, loc := range p.re.FindAllStringIndex(text, -1) {
			m := SecretMatch{Kind: p.kind, Start: loc[0], End: loc[1]}
			if !overlaps(m, found) {
				found = append(found, m)
			}
		}
	}
	for _, loc := range candidate.FindAllStringIndex(text, -1) {
		s := text[loc[0]:loc[1]]
		m := SecretMatch{Kind: "high-entropy", Start: loc[0], End: loc[1]}
		classes := 0
		for _, re := range []*regexp.Regexp{lower, upper, digit} {
			if re.MatchString(s) {
				classes++
			}
		}
		// Mixed character classes and high entropy: unlikely to be a word, path or plain hex hash.
		if classes >= 3 && Entropy(s) >= 4.3 && !overlaps(m, found) {
			found = append(found, m)
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].Start < found[j].Start })
	return found
}

// RedactSecrets replaces each secret with [REDACTED:<kind>].
func RedactSecrets(text string) (string, []SecretMatch) {
	matches := FindSecrets(text)
	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(text[last:m.Start])
		b.WriteString("[REDACTED:" + m.Kind + "]")
		last = m.End
	}
	b.WriteString(text[last:])
	return b.String(), matches
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}
