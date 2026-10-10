package markdowntext_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/markdowntext"
	"github.com/stretchr/testify/require"
)

func TestSplitCodeSpans(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantCode []string // texts of isCode spans, in order
		wantText []string // texts of non-code spans, in order
	}{
		{
			name:     "no backticks",
			input:    "hello world",
			wantCode: nil,
			wantText: []string{"hello world"},
		},
		{
			name:     "inline code",
			input:    "use `foo` here",
			wantCode: []string{"`foo`"},
			wantText: []string{"use ", " here"},
		},
		{
			name:     "fenced block",
			input:    "before\n```\ncode\n```\nafter",
			wantCode: []string{"```\ncode\n```"},
			wantText: []string{"before\n", "\nafter"},
		},
		{
			name:     "unmatched backtick is plain text",
			input:    "odd ` alone",
			wantCode: nil,
			wantText: []string{"odd ", "` alone"},
		},
		{
			name:     "longer fence not closed by shorter",
			input:    "````code````",
			wantCode: []string{"````code````"},
			wantText: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spans := markdowntext.SplitCodeSpans(c.input)
			var gotCode, gotText []string
			for _, s := range spans {
				if s.IsCode {
					gotCode = append(gotCode, s.Text)
				} else {
					gotText = append(gotText, s.Text)
				}
			}
			if !reflect.DeepEqual(gotCode, c.wantCode) {
				t.Errorf("splitCodeSpans(%q) code spans = %v, want %v", c.input, gotCode, c.wantCode)
			}
			if !reflect.DeepEqual(gotText, c.wantText) {
				t.Errorf("splitCodeSpans(%q) text spans = %v, want %v", c.input, gotText, c.wantText)
			}
		})
	}
}

func TestOutsideCode(t *testing.T) {
	require.Equal(t, []string{"a ", " b ", " c"}, markdowntext.OutsideCode("a `x` b ```\ny\n``` c"))
}

func TestReplaceOutsideCode(t *testing.T) {
	upper := func(part string) string { return strings.ToUpper(part) }
	require.Equal(t, "A `x` B", markdowntext.ReplaceOutsideCode("a `x` b", upper))
}
