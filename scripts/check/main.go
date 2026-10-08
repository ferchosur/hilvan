// Command check runs every gate before a commit: gofmt, go vet and go test.
// Usage: go run ./scripts/check (replaces `npm run check` once plan 002 ends).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	out, err := exec.Command("gofmt", "-l", ".").Output()
	if err != nil {
		fail("gofmt", err)
	}
	if files := strings.TrimSpace(string(out)); files != "" {
		fmt.Fprintf(os.Stderr, "gofmt: unformatted files:\n%s\n", files)
		os.Exit(1)
	}
	for _, step := range [][]string{{"go", "vet", "./..."}, {"go", "test", "./..."}} {
		cmd := exec.Command(step[0], step[1:]...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fail(strings.Join(step, " "), err)
		}
	}
	fmt.Println("check: ok")
}

func fail(step string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", step, err)
	os.Exit(1)
}
