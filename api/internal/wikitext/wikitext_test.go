package wikitext_test

import (
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/wikitext"
	"github.com/stretchr/testify/require"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Konvencie backendu":            "konvencie-backendu",
		"Prehľad systému":               "prehlad-systemu",
		"  ADR-004: Branch per run!  ":  "adr-004-branch-per-run",
		"Ťažké ŽLUŤOUČKÉ kôň":           "tazke-zlutoucke-kon",
		"---":                           "",
		"Release 2.0 / notes & roadmap": "release-2-0-notes-roadmap",
	}
	for input, want := range cases {
		require.Equal(t, want, wikitext.Slugify(input), input)
	}
}

func TestSlugify_RespectsMaxLength(t *testing.T) {
	slug := wikitext.Slugify(strings.Repeat("ab ", 100))
	require.LessOrEqual(t, len(slug), wikitext.MaxSlugLength)
	require.False(t, strings.HasSuffix(slug, "-"))
}

func TestParseLinks_FindsUniqueTargetsOutsideCode(t *testing.T) {
	body := strings.Join([]string{
		"See [[Prehľad systému]] and [[prehlad-systemu|the overview]].",
		"Shared: [[spolocne:Ako píšeme commity]] and [[shared:security]].",
		"Inline `[[not a link]]` and fenced:",
		"```",
		"[[also not]]",
		"```",
		"Empty [[ ]] is ignored.",
		"Headings [[Prehľad systému#Vrstvy]], [[shared:security#Kľúče|keys]] and [[#Local heading]].",
	}, "\n")

	links := wikitext.ParseLinks(body)

	require.Equal(t, []wikitext.Link{
		{Shared: false, Slug: "prehlad-systemu"},
		{Shared: true, Slug: "ako-piseme-commity"},
		{Shared: true, Slug: "security"},
	}, links)
}

func TestEstimateTokens(t *testing.T) {
	require.Equal(t, 0, wikitext.EstimateTokens(""))
	require.Equal(t, 1, wikitext.EstimateTokens("abcd"))
	require.Equal(t, 2, wikitext.EstimateTokens("abcde"))
}

func TestToPrefixQuery_MatchesEveryWordByItsStart(t *testing.T) {
	cases := map[string]string{
		"Hel":                 "hel:*",
		"  prehľad  SYS ":     "prehľad:* & sys:*",
		"worktree-retention!": "worktree:* & retention:*",
		"a & b | !c:*":        "a:* & b:* & c:*",
		"&|!()":               "",
	}
	for input, expected := range cases {
		require.Equal(t, expected, wikitext.ToPrefixQuery(input), input)
	}
}
