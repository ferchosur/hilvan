// Package jsfmt reproduces JavaScript formatting rules that Hilvan's stored data and messages
// depend on, so the Go implementation stays byte-compatible with the TypeScript one (plan 002,
// verified in spike 006).
package jsfmt

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Number formats a float64 like ECMAScript Number::toString (and JSON.stringify).
func Number(f float64) string {
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
	e := strconv.FormatFloat(f, 'e', -1, 64) // d.dddde±XX with the shortest round-trip digits
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

// String quotes like JSON.stringify: only ", \ and C0 controls are escaped.
func String(s string) string {
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

// Round is JavaScript's Math.round: halves round towards +∞.
func Round(x float64) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	return r
}

// Less orders strings like JavaScript's < (by UTF-16 code units, not UTF-8 bytes).
func Less(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// Canonical serialises a decoded JSON value (nil, bool, float64, string, []any,
// map[string]any) with sorted keys, exactly like canonical() in src/store/audit.ts.
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
		return Number(x)
	case int:
		return Number(float64(x))
	case int64:
		return Number(float64(x))
	case string:
		return String(x)
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
		sort.Slice(keys, func(i, j int) bool { return Less(keys[i], keys[j]) })
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = String(k) + ":" + Canonical(x[k])
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	panic(fmt.Sprintf("jsfmt.Canonical: unsupported %T", v))
}

// UTF16Len counts UTF-16 code units, which is what JavaScript's String.length reports.
func UTF16Len(s string) int {
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

// ToFixed is Number.prototype.toFixed: the exact binary value rounded to digits decimals, ties
// away from zero (Go's strconv rounds ties to even: 0.125 → "0.12" instead of "0.13").
func ToFixed(x float64, digits int) string {
	if math.IsNaN(x) {
		return "NaN"
	}
	if math.Abs(x) >= 1e21 || math.IsInf(x, 0) {
		return Number(x)
	}
	sign := ""
	if x < 0 {
		sign, x = "-", -x
	}
	exact := new(big.Float).SetPrec(2048).SetFloat64(x).Text('f', 1100) // float64 is exact in decimal
	intPart, frac, _ := strings.Cut(exact, ".")
	frac += strings.Repeat("0", digits+1)
	keep, next := frac[:digits], frac[digits]
	n := new(big.Int)
	n.SetString(intPart+keep, 10)
	if next >= '5' {
		n.Add(n, big.NewInt(1))
	}
	s := n.String()
	if digits > 0 {
		if len(s) <= digits {
			s = strings.Repeat("0", digits-len(s)+1) + s
		}
		s = s[:len(s)-digits] + "." + s[len(s)-digits:]
	}
	return sign + s // (-0.0001).toFixed(2) is "-0.00" in JS too
}
