package wikimerge

import (
	"slices"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

type ChunkKind string

const (
	ChunkStable   ChunkKind = "stable"
	ChunkConflict ChunkKind = "conflict"
)

type Chunk struct {
	Kind   ChunkKind `json:"kind"`
	Lines  []string  `json:"lines,omitempty"`
	Base   []string  `json:"base,omitempty"`
	Mine   []string  `json:"mine,omitempty"`
	Theirs []string  `json:"theirs,omitempty"`
}

type Result struct {
	Chunks    []Chunk `json:"chunks"`
	Conflicts int     `json:"conflicts"`
}

type resultBuilder struct {
	result Result
}

func (r Result) HasConflicts() bool {
	return r.Conflicts > 0
}

func (r Result) Text() string {
	lines := make([]string, 0)
	for _, chunk := range r.Chunks {
		if chunk.Kind == ChunkStable {
			lines = append(lines, chunk.Lines...)
		}
	}
	return strings.Join(lines, "\n")
}

func Lines(text string) []string {
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}

func Merge(base, mine, theirs string) Result {
	baseLines := Lines(base)
	mineLines := Lines(mine)
	theirsLines := Lines(theirs)

	toMine := matchIndex(baseLines, mineLines)
	toTheirs := matchIndex(baseLines, theirsLines)

	builder := &resultBuilder{}
	idxBase, idxMine, idxTheirs := 0, 0, 0
	for {
		if idxBase < len(baseLines) && toMine[idxBase] == idxMine && toTheirs[idxBase] == idxTheirs {
			builder.stable(baseLines[idxBase : idxBase+1])
			idxBase++
			idxMine++
			idxTheirs++
			continue
		}

		syncBase, syncMine, syncTheirs := len(baseLines), len(mineLines), len(theirsLines)
		for candidate := idxBase; candidate < len(baseLines); candidate++ {
			if toMine[candidate] >= idxMine && toTheirs[candidate] >= idxTheirs {
				syncBase, syncMine, syncTheirs = candidate, toMine[candidate], toTheirs[candidate]
				break
			}
		}

		builder.resolve(
			baseLines[idxBase:syncBase],
			mineLines[idxMine:syncMine],
			theirsLines[idxTheirs:syncTheirs],
		)
		idxBase, idxMine, idxTheirs = syncBase, syncMine, syncTheirs
		if idxBase >= len(baseLines) {
			break
		}
	}
	return builder.result
}

func MergeField(base, mine, theirs string) string {
	if mine == base {
		return theirs
	}
	return mine
}

func UnifiedDiff(from, to, fromName, toName string) (string, error) {
	return difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(from),
		B:        difflib.SplitLines(to),
		FromFile: fromName,
		ToFile:   toName,
		Context:  3,
	})
}

func matchIndex(base, other []string) []int {
	index := make([]int, len(base))
	for i := range index {
		index[i] = -1
	}
	matcher := difflib.NewMatcherWithJunk(base, other, false, nil)
	for _, block := range matcher.GetMatchingBlocks() {
		for offset := 0; offset < block.Size; offset++ {
			index[block.A+offset] = block.B + offset
		}
	}
	return index
}

func (b *resultBuilder) stable(lines []string) {
	if len(lines) == 0 {
		return
	}
	last := len(b.result.Chunks) - 1
	if last >= 0 && b.result.Chunks[last].Kind == ChunkStable {
		b.result.Chunks[last].Lines = append(b.result.Chunks[last].Lines, lines...)
		return
	}
	b.result.Chunks = append(b.result.Chunks, Chunk{Kind: ChunkStable, Lines: slices.Clone(lines)})
}

func (b *resultBuilder) resolve(base, mine, theirs []string) {
	switch {
	case slices.Equal(mine, base):
		b.stable(theirs)
	case slices.Equal(theirs, base), slices.Equal(mine, theirs):
		b.stable(mine)
	default:
		b.result.Chunks = append(b.result.Chunks, Chunk{
			Kind:   ChunkConflict,
			Base:   slices.Clone(base),
			Mine:   slices.Clone(mine),
			Theirs: slices.Clone(theirs),
		})
		b.result.Conflicts++
	}
}
