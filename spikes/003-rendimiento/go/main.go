// Spike 003 — Go slice of Hilvan (stdlib only, Go 1.17 compatible).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

var texts = []string{
	"Refactor the Codex adapter to normalise JSONL events and add tests",
	"Fix bug in login flow when the token expires",
	"Document the API in the README with examples",
	"Investigate terminal UI alternatives for low resource machines",
	"Add support for SSH destinations in the launcher",
	"Review the authentication pull request for security issues",
	"Increase unit test coverage of the policy evaluator",
	"Implement export of task history to CSV files",
}

const k = 7
const buckets = 65536

func fnv1a(s string) uint32 {
	h := uint32(0x811c9dc5)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 0x01000193
	}
	return h
}

func tokenize(t string) []string {
	out := []string{}
	for _, w := range strings.FieldsFunc(strings.ToLower(t), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	}) {
		if len(w) < 2 {
			continue
		}
		if len(w) > 6 {
			w = w[:6]
		}
		out = append(out, w)
	}
	return out
}

func features(t string) []string {
	toks := tokenize(t)
	f := make([]string, 0, 2*len(toks))
	for i, tok := range toks {
		f = append(f, "w:"+tok)
		if i+1 < len(toks) {
			f = append(f, "b:"+tok+"_"+toks[i+1])
		}
	}
	return f
}

func weightsFor(b uint32) [k]float64 {
	var w [k]float64
	for c := 0; c < k; c++ {
		w[c] = float64((uint32(uint64(b)*uint64(c+1)*2654435761))%1000)/1000 - 0.5
	}
	return w
}

func criterio(n int) {
	W := map[uint32][k]float64{}
	for _, t := range texts {
		for _, f := range features(t) {
			b := fnv1a(f) % buckets
			W[b] = weightsFor(b)
		}
	}
	check := 0
	for i := 0; i < n; i++ {
		var z [k]float64
		for _, f := range features(texts[i%8] + " " + strconv.Itoa(i%100)) {
			if w, ok := W[fnv1a(f)%buckets]; ok {
				for c := 0; c < k; c++ {
					z[c] += w[c]
				}
			}
		}
		m := math.Inf(-1)
		for _, v := range z {
			m = math.Max(m, v)
		}
		best := 0
		var p [k]float64
		for c := 0; c < k; c++ {
			p[c] = math.Exp(z[c] - m)
			if p[c] > p[best] {
				best = c
			}
		}
		check += best
	}
	fmt.Printf("criterio n=%d check=%d\n", n, check)
}

type event struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message *struct {
		Content []struct {
			Type string `json:"type"`
		} `json:"content"`
	} `json:"message"`
	Usage *struct {
		In  int64 `json:"input_tokens"`
		Out int64 `json:"output_tokens"`
	} `json:"usage"`
}

func readLines(r io.Reader) {
	var lines, session, text, limits, results, tokens int64
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		lines++
		var e event
		if err := json.Unmarshal(line, &e); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		switch e.Type {
		case "system":
			if e.Subtype == "init" {
				session++
			}
		case "assistant":
			if e.Message != nil {
				for _, c := range e.Message.Content {
					if c.Type == "text" {
						text++
					}
				}
			}
		case "rate_limit_event":
			limits++
		case "result":
			results++
			if e.Usage != nil {
				tokens += e.Usage.In + e.Usage.Out
			}
		}
	}
	fmt.Printf("lines=%d session=%d text=%d limits=%d results=%d tokens=%d\n", lines, session, text, limits, results, tokens)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: version | criterio N | jsonl FILE | spawn FILE")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("hilvan-go 0.0.0")
	case "criterio":
		n, _ := strconv.Atoi(os.Args[2])
		criterio(n)
	case "jsonl":
		f, err := os.Open(os.Args[2])
		if err != nil {
			panic(err)
		}
		defer f.Close()
		readLines(f)
	case "spawn":
		cmd := exec.Command("cat", os.Args[2])
		out, _ := cmd.StdoutPipe()
		if err := cmd.Start(); err != nil {
			panic(err)
		}
		readLines(out)
		_ = cmd.Wait()
	}
}
