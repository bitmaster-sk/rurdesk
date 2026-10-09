package attachmenttext_test

import (
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/attachmenttext"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseIds(t *testing.T) {
	first := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	second := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	cases := []struct {
		name string
		body string
		want []uuid.UUID
	}{
		{"none", "hello world", nil},
		{"image", "![shot](attachment:11111111-1111-1111-1111-111111111111)", []uuid.UUID{first}},
		{"file", "[doc.pdf](attachment:11111111-1111-1111-1111-111111111111)", []uuid.UUID{first}},
		{
			"multiple in order",
			"[a](attachment:22222222-2222-2222-2222-222222222222) ![b](attachment:11111111-1111-1111-1111-111111111111)",
			[]uuid.UUID{second, first},
		},
		{
			"dedupes",
			"![a](attachment:11111111-1111-1111-1111-111111111111) ![a](attachment:11111111-1111-1111-1111-111111111111)",
			[]uuid.UUID{first},
		},
		{"ignores invalid uuid", "![a](attachment:not-a-uuid)", nil},
		{"ignores plain link", "[docs](http://x)", nil},
		{"inline code ignored", "`![a](attachment:11111111-1111-1111-1111-111111111111)`", nil},
		{"fenced block ignored", "```\n![a](attachment:11111111-1111-1111-1111-111111111111)\n```", nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, attachmenttext.ParseIds(testCase.body))
		})
	}
}

func TestStripLinks(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"image", "look ![shot](attachment:11111111-1111-1111-1111-111111111111) here", "look [shot] here"},
		{"file", "[doc.pdf](attachment:11111111-1111-1111-1111-111111111111)", "[doc.pdf]"},
		{"plain link kept", "[docs](http://x)", "[docs](http://x)"},
		{"no links", "hello", "hello"},
		{"code kept", "`![shot](attachment:11111111-1111-1111-1111-111111111111)`", "`![shot](attachment:11111111-1111-1111-1111-111111111111)`"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, attachmenttext.StripLinks(testCase.input))
		})
	}
}
