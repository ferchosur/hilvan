package jsfmt

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

type textGolden struct {
	Canonical []struct {
		In  any    `json:"in"`
		Out string `json:"out"`
	} `json:"canonical"`
}

// Golden cases from the TypeScript implementation (scripts/parity/gen-g1.ts).
func TestCanonicalMatchesTypeScript(t *testing.T) {
	raw, err := os.ReadFile("testdata/text.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g textGolden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.Canonical {
		if got := Canonical(c.In); got != c.Out {
			t.Errorf("got  %s\nwant %s", got, c.Out)
		}
	}
}

func TestNumber(t *testing.T) {
	a, b := 0.1, 0.2 // variables: constant arithmetic in Go is exact
	cases := map[float64]string{
		0: "0", math.Copysign(0, -1): "0", 1: "1", -1.5: "-1.5", a + b: "0.30000000000000004",
		1e21: "1e+21", 1e20: "100000000000000000000", 1e-6: "0.000001", 1e-7: "1e-7", 1.5e-7: "1.5e-7",
		5e-324: "5e-324", math.MaxFloat64: "1.7976931348623157e+308", math.Inf(1): "null",
	}
	for f, want := range cases {
		if got := Number(f); got != want {
			t.Errorf("%v: got %s want %s", f, got, want)
		}
	}
}

func TestRound(t *testing.T) {
	cases := map[float64]float64{0.5: 1, -0.5: 0, 1.5: 2, -1.5: -1, 2.4: 2, -2.6: -3, 0.49999999999999994: 0}
	for in, want := range cases {
		if got := Round(in); got != want {
			t.Errorf("Round(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestString(t *testing.T) {
	if got := String("a\"b\\c\n <&>\x01"); got != `"a\"b\\c\n`+" "+`<&>\u0001"` {
		t.Errorf("got %s", got)
	}
}

func TestToFixedMatchesJavaScript(t *testing.T) {
	cases := []struct {
		x    float64
		d    int
		want string
	}{
		{0.125, 2, "0.13"}, {0.0123, 4, "0.0123"}, {1.005, 2, "1.00"}, {2.5, 0, "3"}, {-1.5, 0, "-2"},
		{-0.0001, 2, "-0.00"}, {0, 2, "0.00"}, {123.456, 1, "123.5"}, {0.3, 3, "0.300"}, {1e-7, 4, "0.0000"},
		{0.375, 2, "0.38"}, {-2.675, 2, "-2.67"},
	}
	for _, c := range cases {
		if got := ToFixed(c.x, c.d); got != c.want {
			t.Errorf("(%v).toFixed(%d) = %q, want %q", c.x, c.d, got, c.want)
		}
	}
}
