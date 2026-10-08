package service

import (
	"encoding/json"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikitext"
)

// The stage_execute webhook is capped at 1 MiB by the gateway, so the wiki block stays well below it
// whatever token limit a project sets. Bytes are counted as JSON, where <, > and & take six bytes each.
const (
	wikiPromptMaxTokens = 60000
	wikiPromptMaxBytes  = 400000
)

const (
	wikiPromptReasonAlways = "always"
	wikiPromptReasonLinked = "linked"
)

type wikiPromptInput struct {
	Pages           []*model.WikiAgentPageRow
	IdsLinked       map[int64]bool
	IdInstanceSpace int64
	TokenLimit      int
}

type wikiPromptRead struct {
	Page   *model.WikiAgentPageRow
	Source constants.WikiReadSource
	Tokens int
}

func buildWikiPrompt(input wikiPromptInput) (model.WikiPromptContext, []wikiPromptRead) {
	budget := min(input.TokenLimit, wikiPromptMaxTokens)
	byteBudget := wikiPromptMaxBytes
	prompt := model.WikiPromptContext{Pages: []model.WikiPromptPage{}, Index: []model.WikiPromptIndexEntry{}}
	reads := []wikiPromptRead{}
	ordered := orderForPrompt(input.Pages, input.IdInstanceSpace)

	included := map[int64]bool{}
	overLimit := map[int64]bool{}
	for _, page := range promptCandidates(ordered, input.IdsLinked) {
		if page.Body == nil {
			continue
		}
		entry := model.WikiPromptPage{
			Slug:    agentSlug(page, input.IdInstanceSpace),
			Title:   page.Title,
			Version: page.VersionNo,
			Reason:  promptReason(page),
			Body:    *page.Body,
		}
		cost := wikiPromptPageTokens(entry)
		size := encodedSize(entry)
		if cost > budget || size > byteBudget {
			overLimit[page.IdPage] = true
			continue
		}
		budget -= cost
		byteBudget -= size
		included[page.IdPage] = true
		prompt.Pages = append(prompt.Pages, entry)
		reads = append(reads, wikiPromptRead{Page: page, Source: promptSource(entry.Reason), Tokens: cost})
	}

	for _, page := range indexOrder(ordered, overLimit) {
		if included[page.IdPage] {
			continue
		}
		entry := model.WikiPromptIndexEntry{
			Slug:      agentSlug(page, input.IdInstanceSpace),
			Title:     page.Title,
			Summary:   page.Summary,
			OverLimit: overLimit[page.IdPage],
		}
		cost := wikiPromptIndexTokens(entry)
		size := encodedSize(entry)
		if cost > budget || size > byteBudget {
			prompt.IndexMore++
			continue
		}
		budget -= cost
		byteBudget -= size
		prompt.Index = append(prompt.Index, entry)
		reads = append(reads, wikiPromptRead{Page: page, Source: constants.WikiReadPromptIndex, Tokens: cost})
	}
	return prompt, reads
}

func wikiPromptPageTokens(page model.WikiPromptPage) int {
	header := fmt.Sprintf("### %s (%s · v%d · %s)\n", page.Title, page.Slug, page.Version, page.Reason)
	return wikitext.EstimateTokens(header + page.Body + "\n")
}

func wikiPromptIndexTokens(entry model.WikiPromptIndexEntry) int {
	return wikitext.EstimateTokens(fmt.Sprintf("- %s — %s — %s (not loaded, over limit)\n", entry.Slug, entry.Title, entry.Summary))
}

func encodedSize(value any) int {
	encoded, err := json.Marshal(value)
	if err != nil {
		return wikiPromptMaxBytes + 1
	}
	return len(encoded)
}

func agentSlug(page *model.WikiAgentPageRow, idInstanceSpace int64) string {
	if page.IdSpace == idInstanceSpace {
		return agentSlugIn(constants.WikiSpaceInstance, page.Slug)
	}
	return agentSlugIn(constants.WikiSpaceProject, page.Slug)
}

func orderForPrompt(pages []*model.WikiAgentPageRow, idInstanceSpace int64) []*model.WikiAgentPageRow {
	ordered := make([]*model.WikiAgentPageRow, 0, len(pages))
	for _, page := range pages {
		if page.IdSpace != idInstanceSpace {
			ordered = append(ordered, page)
		}
	}
	for _, page := range pages {
		if page.IdSpace == idInstanceSpace {
			ordered = append(ordered, page)
		}
	}
	return ordered
}

func promptCandidates(ordered []*model.WikiAgentPageRow, idsLinked map[int64]bool) []*model.WikiAgentPageRow {
	candidates := make([]*model.WikiAgentPageRow, 0)
	for _, page := range ordered {
		if idsLinked[page.IdPage] {
			candidates = append(candidates, page)
		}
	}
	for _, page := range ordered {
		if !idsLinked[page.IdPage] && page.AgentAccess == constants.WikiAgentAccessAlways {
			candidates = append(candidates, page)
		}
	}
	return candidates
}

func indexOrder(ordered []*model.WikiAgentPageRow, overLimit map[int64]bool) []*model.WikiAgentPageRow {
	result := make([]*model.WikiAgentPageRow, 0, len(ordered))
	for _, page := range ordered {
		if overLimit[page.IdPage] {
			result = append(result, page)
		}
	}
	for _, page := range ordered {
		if !overLimit[page.IdPage] {
			result = append(result, page)
		}
	}
	return result
}

func promptReason(page *model.WikiAgentPageRow) string {
	if page.AgentAccess == constants.WikiAgentAccessAlways {
		return wikiPromptReasonAlways
	}
	return wikiPromptReasonLinked
}

func promptSource(reason string) constants.WikiReadSource {
	if reason == wikiPromptReasonAlways {
		return constants.WikiReadPromptAlways
	}
	return constants.WikiReadPromptLinked
}
