package common

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReadLines(t *testing.T) {
	longUnderLimit := strings.Repeat("a", 100*1024)
	tests := []struct {
		name         string
		input        string
		maxLineBytes int
		wantLines    []string
		wantSkipped  int
	}{
		{
			name:         "skips a line over the limit and keeps reading",
			input:        "short\nmuch longer than ten\nafter\n",
			maxLineBytes: 10,
			wantLines:    []string{"short", "after"},
			wantSkipped:  1,
		},
		{
			name:         "keeps a line of exactly the limit",
			input:        "0123456789\n",
			maxLineBytes: 10,
			wantLines:    []string{"0123456789"},
		},
		{
			name:         "skips an oversized line spanning many buffer fills",
			input:        strings.Repeat("x", 300*1024) + "\nnext\n",
			maxLineBytes: 1024,
			wantLines:    []string{"next"},
			wantSkipped:  1,
		},
		{
			name:         "keeps a line longer than the read buffer but under the limit",
			input:        longUnderLimit + "\n",
			maxLineBytes: 200 * 1024,
			wantLines:    []string{longUnderLimit},
		},
		{
			name:         "returns the last line without a trailing newline",
			input:        "first\nlast",
			maxLineBytes: 10,
			wantLines:    []string{"first", "last"},
		},
		{
			name:         "skips an oversized last line without a trailing newline",
			input:        "first\n" + strings.Repeat("y", 20),
			maxLineBytes: 10,
			wantLines:    []string{"first"},
			wantSkipped:  1,
		},
		{
			name:         "passes empty lines through",
			input:        "a\n\nb\n",
			maxLineBytes: 10,
			wantLines:    []string{"a", "", "b"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var lines []string
			skipped, err := ReadLines(strings.NewReader(test.input), test.maxLineBytes, func(line []byte) {
				lines = append(lines, string(line))
			})
			if err != nil {
				t.Fatalf("ReadLines error = %v", err)
			}
			if skipped != test.wantSkipped {
				t.Errorf("skipped = %d, want %d", skipped, test.wantSkipped)
			}
			if strings.Join(lines, "|") != strings.Join(test.wantLines, "|") || len(lines) != len(test.wantLines) {
				t.Errorf("lines = %q, want %q", lines, test.wantLines)
			}
		})
	}
}

func TestReadLinesReturnsTheReadError(t *testing.T) {
	readErr := errors.New("pipe broke")
	_, err := ReadLines(iotest.ErrReader(readErr), 10, func([]byte) {})
	if !errors.Is(err, readErr) {
		t.Fatalf("err = %v, want %v", err, readErr)
	}
}
