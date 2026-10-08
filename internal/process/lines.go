// Package process runs external commands without a shell (SR-004) and splits their JSONL
// output with hard limits (SR-009).
package process

import (
	"errors"
	"strings"
)

// LineLimitError reports a line or a stream above its limit.
type LineLimitError struct{ Reason string } // "line" or "total"

func (e *LineLimitError) Error() string {
	if e.Reason == "line" {
		return "line exceeds the maximum length"
	}
	return "stream exceeds the maximum size"
}

// IsLineLimit reports whether err is a LineLimitError.
func IsLineLimit(err error) bool {
	var l *LineLimitError
	return errors.As(err, &l)
}

// Default limits: 1 MiB per line, 256 MiB per stream.
const (
	DefaultMaxLine  = 1024 * 1024
	DefaultMaxTotal = 256 * 1024 * 1024
)

// LineSplitter feeds complete lines to OnLine and never buffers more than MaxLine bytes of a
// partial line. Blank lines are dropped and a trailing \r is removed.
type LineSplitter struct {
	OnLine            func(line string)
	MaxLine, MaxTotal int
	partial           strings.Builder
	total             int
}

// NewLineSplitter returns a splitter with the default limits.
func NewLineSplitter(onLine func(string)) *LineSplitter {
	return &LineSplitter{OnLine: onLine, MaxLine: DefaultMaxLine, MaxTotal: DefaultMaxTotal}
}

// Push adds a chunk of output.
func (s *LineSplitter) Push(chunk string) error {
	s.total += len(chunk)
	if s.total > s.MaxTotal {
		return &LineLimitError{Reason: "total"}
	}
	for {
		nl := strings.IndexByte(chunk, '\n')
		if nl < 0 {
			break
		}
		line := s.partial.String() + chunk[:nl]
		s.partial.Reset()
		if len(line) > s.MaxLine {
			return &LineLimitError{Reason: "line"}
		}
		s.emit(line)
		chunk = chunk[nl+1:]
	}
	s.partial.WriteString(chunk)
	if s.partial.Len() > s.MaxLine {
		return &LineLimitError{Reason: "line"}
	}
	return nil
}

// End flushes a trailing line without a newline.
func (s *LineSplitter) End() {
	if s.partial.Len() > 0 {
		line := s.partial.String()
		s.partial.Reset()
		s.emit(line)
	}
}

func (s *LineSplitter) emit(line string) {
	line = strings.TrimSuffix(line, "\r")
	if strings.TrimSpace(line) != "" {
		s.OnLine(line)
	}
}
