package main

import (
	"bytes"
	"testing"

	"github.com/ferchosur/hilvan/internal/version"
)

func TestVersion(t *testing.T) {
	for _, flag := range []string{"-V", "--version"} {
		var out, errOut bytes.Buffer
		if code := run([]string{flag}, &out, &errOut); code != 0 {
			t.Fatalf("%s: exit %d", flag, code)
		}
		if got, want := out.String(), version.Version+"\n"; got != want {
			t.Fatalf("%s: got %q, want %q", flag, got, want)
		}
	}
}

func TestUnknownCommandFails(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"task"}, &out, &errOut); code == 0 {
		t.Fatal("expected a non-zero exit for a command not yet ported")
	}
	if errOut.Len() == 0 {
		t.Fatal("expected an error message")
	}
}
