package controller

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func Test_truncate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"shorter than limit", "hello", 10, "hello"},
		{"exactly at limit", "hello", 5, "hello"},
		{"ascii over limit", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
		{
			name: "diacritics under rune limit but over byte limit",
			in:   "žltý kôň",
			max:  8,
			want: "žltý kôň",
		},
		{
			name: "diacritics over limit cut on rune boundary",
			in:   strings.Repeat("ť", 10),
			max:  5,
			want: strings.Repeat("ť", 5) + "...",
		},
		{
			name: "emoji cut on rune boundary",
			in:   strings.Repeat("🙂", 4),
			max:  2,
			want: strings.Repeat("🙂", 2) + "...",
		},
		{
			name: "slovak sentence at notification limit",
			in:   strings.Repeat("ďáčšž", 60),
			max:  200,
			want: strings.Repeat("ďáčšž", 40) + "...",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncate(c.in, c.max)
			assert.Equal(t, c.want, got)
			assert.True(t, utf8.ValidString(got), "must not cut a rune in half")
			assert.LessOrEqual(t, utf8.RuneCountInString(strings.TrimSuffix(got, "...")), c.max)
		})
	}
}
