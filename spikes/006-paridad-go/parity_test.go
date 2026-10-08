package spike006

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func lines(t *testing.T, file string, fn func([]byte)) {
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		fn(sc.Bytes())
	}
}

func TestNumbers(t *testing.T) {
	n, bad := 0, 0
	lines(t, "testdata/numbers.jsonl", func(b []byte) {
		var c struct{ Bits, Out string }
		json.Unmarshal(b, &c)
		raw, _ := hex.DecodeString(c.Bits)
		f := math.Float64frombits(binary.BigEndian.Uint64(raw))
		n++
		if got := JSNumber(f); got != c.Out {
			bad++
			if bad <= 5 {
				t.Errorf("%v: got %s want %s", f, got, c.Out)
			}
		}
	})
	t.Logf("numbers: %d cases, %d mismatches", n, bad)
}

func TestCanonical(t *testing.T) {
	n, bad := 0, 0
	lines(t, "testdata/canonical.jsonl", func(b []byte) {
		var c struct {
			In  any
			Out string
		}
		json.Unmarshal(b, &c)
		n++
		if got := Canonical(c.In); got != c.Out {
			bad++
			if bad <= 5 {
				t.Errorf("got %s\nwant %s", got, c.Out)
			}
		}
	})
	t.Logf("canonical: %d cases, %d mismatches", n, bad)
}

func TestCriterio(t *testing.T) {
	n, bad := 0, 0
	lines(t, "testdata/criterio.jsonl", func(b []byte) {
		var c struct {
			Text   string
			Fnv    uint32
			Tokens []string
		}
		json.Unmarshal(b, &c)
		n++
		if FNV1a(c.Text) != c.Fnv || !reflect.DeepEqual(Tokenize(c.Text), c.Tokens) {
			bad++
			if bad <= 5 {
				t.Errorf("%q: fnv %d/%d tokens %v/%v", c.Text, FNV1a(c.Text), c.Fnv, Tokenize(c.Text), c.Tokens)
			}
		}
	})
	t.Logf("criterio: %d cases, %d mismatches", n, bad)
}
