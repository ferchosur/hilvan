// Spike 006 (desechable): prototipos Go de las piezas con riesgo de paridad.
package spike006

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"golang.org/x/text/unicode/norm"
)

// JSNumber formats a float64 like ECMAScript Number::toString (as JSON.stringify does).
func JSNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	if f == 0 {
		return "0"
	}
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}
	e := strconv.FormatFloat(f, 'e', -1, 64) // d.dddde±XX, shortest round-trip digits
	mant, expStr, _ := strings.Cut(e, "e")
	digits := strings.Replace(mant, ".", "", 1)
	exp, _ := strconv.Atoi(expStr)
	k, n := len(digits), exp+1
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}
	es := "+"
	if n-1 < 0 {
		es = "-"
	}
	ex := strconv.Itoa(int(math.Abs(float64(n - 1))))
	if k == 1 {
		return sign + digits + "e" + es + ex
	}
	return sign + digits[:1] + "." + digits[1:] + "e" + es + ex
}

// JSString quotes like JSON.stringify: only ", \ and C0 controls are escaped.
func JSString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// utf16Less orders keys like JS string comparison (by UTF-16 code units).
func utf16Less(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// Canonical mirrors src/store/audit.ts canonical() for values decoded with UseNumber=false.
func Canonical(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return JSNumber(x)
	case string:
		return JSString(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = Canonical(e)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return utf16Less(keys[i], keys[j]) })
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = JSString(k) + ":" + Canonical(x[k])
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	panic(fmt.Sprintf("unsupported %T", v))
}

// FNV1a hashes UTF-16 code units, like the TS charCodeAt loop.
func FNV1a(s string) uint32 {
	h := uint32(0x811c9dc5)
	for _, u := range utf16.Encode([]rune(s)) {
		h ^= uint32(u)
		h *= 0x01000193
	}
	return h
}

var (
	nonAlnum  = regexp.MustCompile(`[^a-z0-9]+`)
	stopwords = map[string]bool{}
)

func init() {
	for _, w := range strings.Fields("a al algo and are as at be by con de del el en es esta este for from ha in is it la las lo los " +
		"of on or para por que se sin su the to un una y que we you our nuestro") {
		stopwords[w] = true
	}
}

// Tokenize mirrors src/criterio/features.ts tokenize().
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
		if len(raw) > 6 {
			raw = raw[:6]
		}
		out = append(out, raw)
	}
	return out
}
