package controller

import (
	"reflect"
	"testing"
)

func Test_parseMentionUserIds(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []int64
	}{
		{"none", "hello world", nil},
		{"single", "cc @[Jan](user:42) please", []int64{42}},
		{"multiple + spaces", "@[Jan Novák](user:1)@[Eva](user:2)", []int64{1, 2}},
		{"dedupes", "@[Jan](user:5) and again @[Jan](user:5)", []int64{5}},
		{"ignores plain markdown link", "see [docs](http://x)", nil},
		{"overflow skipped", "@[x](user:99999999999999999999)", []int64{}},

		{
			name: "fenced block ignored",
			body: "```\n@[Jan](user:1)\n```",
			want: nil,
		},
		{
			name: "inline code ignored",
			body: "here `@[Jan](user:1)` end",
			want: nil,
		},
		{
			name: "outside mention detected, inside fenced ignored",
			body: "@[Eva](user:2) look at this:\n```\n@[Jan](user:1)\n```",
			want: []int64{2},
		},
		{
			name: "outside mention detected, inside inline code ignored",
			body: "use `@[Jan](user:1)` token or mention @[Eva](user:2) directly",
			want: []int64{2},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseMentionUserIds(c.body)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("parseMentionUserIds(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}

func Test_stripMentionTokens(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single token",
			input: "cc @[Jan](user:42) please review",
			want:  "cc @Jan please review",
		},
		{
			name:  "multiple tokens",
			input: "@[Jan](user:1) and @[Eva Nováková](user:2) should see this",
			want:  "@Jan and @Eva Nováková should see this",
		},
		{
			name:  "plain text unchanged",
			input: "no mentions here, just plain text",
			want:  "no mentions here, just plain text",
		},
		{
			name:  "token with name containing spaces",
			input: "@[Jan Novák](user:99) check this",
			want:  "@Jan Novák check this",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripMentionTokens(c.input)
			if got != c.want {
				t.Fatalf("stripMentionTokens(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func Test_splitCodeSpans(t *testing.T) {
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
			spans := splitCodeSpans(c.input)
			var gotCode, gotText []string
			for _, s := range spans {
				if s.isCode {
					gotCode = append(gotCode, s.text)
				} else {
					gotText = append(gotText, s.text)
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
