package wikimerge_test

import (
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/wikimerge"
	"github.com/stretchr/testify/require"
)

func TestMerge_CombinesChangesInDifferentPlaces(t *testing.T) {
	base := lines("# Title", "", "## Layers", "controller", "", "## Errors", "status from error", "", "## Tests", "run in container")
	mine := lines("# Title", "", "## Layers", "controller calls service", "", "## Errors", "status from error", "", "## Tests", "run in container")
	theirs := lines("# Title", "", "## Layers", "controller", "", "## Errors", "status from error (see #11)", "", "## Tests", "run in container")

	result := wikimerge.Merge(base, mine, theirs)

	require.False(t, result.HasConflicts())
	require.Equal(t, lines("# Title", "", "## Layers", "controller calls service", "", "## Errors", "status from error (see #11)", "", "## Tests", "run in container"), result.Text())
}

func TestMerge_ResultTable(t *testing.T) {
	cases := []struct {
		name      string
		base      string
		mine      string
		theirs    string
		want      string
		conflicts int
	}{
		{name: "nobody changed anything", base: lines("a", "b"), mine: lines("a", "b"), theirs: lines("a", "b"), want: lines("a", "b")},
		{name: "only mine changed", base: lines("a", "b"), mine: lines("a", "B"), theirs: lines("a", "b"), want: lines("a", "B")},
		{name: "only theirs changed", base: lines("a", "b"), mine: lines("a", "b"), theirs: lines("A", "b"), want: lines("A", "b")},
		{name: "same change on both sides", base: lines("a", "b"), mine: lines("a", "X"), theirs: lines("a", "X"), want: lines("a", "X")},
		{name: "both append at the end differently", base: lines("a"), mine: lines("a", "mine"), theirs: lines("a", "theirs"), conflicts: 1},
		{name: "mine appends, theirs edits the top", base: lines("a", "b"), mine: lines("a", "b", "c"), theirs: lines("A", "b"), want: lines("A", "b", "c")},
		{name: "mine deletes a line theirs kept", base: lines("a", "b", "c"), mine: lines("a", "c"), theirs: lines("a", "b", "c", "d"), want: lines("a", "c", "d")},
		{name: "mine deletes a line theirs edited", base: lines("a", "b", "c"), mine: lines("a", "c"), theirs: lines("a", "B", "c"), conflicts: 1},
		{name: "both edit the same line", base: lines("a", "b", "c"), mine: lines("a", "mine", "c"), theirs: lines("a", "theirs", "c"), conflicts: 1},
		{name: "empty base, only mine wrote", base: "", mine: lines("new"), theirs: "", want: lines("new")},
		{name: "repeated blank lines do not confuse alignment", base: lines("a", "", "", "b", "", "", "c"), mine: lines("a", "", "", "B", "", "", "c"), theirs: lines("a", "", "", "b", "", "", "C"), want: lines("a", "", "", "B", "", "", "C")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := wikimerge.Merge(tc.base, tc.mine, tc.theirs)
			require.Equal(t, tc.conflicts, result.Conflicts)
			if tc.conflicts == 0 {
				require.Equal(t, tc.want, result.Text())
			}
		})
	}
}

func TestMerge_ConflictChunkCarriesBothSidesAndKeepsTheRestMerged(t *testing.T) {
	base := lines("intro", "shared", "outro")
	mine := lines("intro edited", "mine", "outro")
	theirs := lines("intro", "theirs", "outro")

	result := wikimerge.Merge(base, mine, theirs)

	require.Equal(t, 1, result.Conflicts)
	var conflict *wikimerge.Chunk
	for i := range result.Chunks {
		if result.Chunks[i].Kind == wikimerge.ChunkConflict {
			conflict = &result.Chunks[i]
		}
	}
	require.NotNil(t, conflict)
	require.Contains(t, conflict.Mine, "mine")
	require.Contains(t, conflict.Theirs, "theirs")
	require.Contains(t, conflict.Base, "shared")
	require.Equal(t, "outro", result.Chunks[len(result.Chunks)-1].Lines[0])
}

func TestMergeField_PrefersTheSideThatChanged(t *testing.T) {
	require.Equal(t, "theirs", wikimerge.MergeField("base", "base", "theirs"))
	require.Equal(t, "mine", wikimerge.MergeField("base", "mine", "base"))
	require.Equal(t, "mine", wikimerge.MergeField("base", "mine", "theirs"))
}

func TestUnifiedDiff_MarksAddedAndRemovedLines(t *testing.T) {
	diff, err := wikimerge.UnifiedDiff("a\nb\n", "a\nc\n", "v1", "v2")
	require.Nil(t, err)
	require.Contains(t, diff, "--- v1")
	require.Contains(t, diff, "+++ v2")
	require.Contains(t, diff, "-b")
	require.Contains(t, diff, "+c")
}

func lines(parts ...string) string {
	return strings.Join(parts, "\n")
}
