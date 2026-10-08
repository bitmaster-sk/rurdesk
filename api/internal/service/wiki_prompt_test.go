package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

const (
	testInstanceSpace int64 = 1
	testProjectSpace  int64 = 2
)

func promptPage(idPage, idSpace int64, slug string, access constants.WikiAgentAccess) *model.WikiAgentPageRow {
	return &model.WikiAgentPageRow{
		IdPage: idPage, IdSpace: idSpace, Slug: slug, Title: strings.ToUpper(slug),
		Summary: "about " + slug, AgentAccess: access, VersionNo: int(idPage),
	}
}

func withBodies(pages []*model.WikiAgentPageRow, bodies map[int64]string) []*model.WikiAgentPageRow {
	for _, page := range pages {
		if body, ok := bodies[page.IdPage]; ok {
			page.Body = &body
		}
	}
	return pages
}

func promptSlugs(prompt model.WikiPromptContext) []string {
	slugs := make([]string, len(prompt.Pages))
	for i, page := range prompt.Pages {
		slugs[i] = page.Slug
	}
	return slugs
}

func indexSlugs(prompt model.WikiPromptContext) []string {
	slugs := make([]string, len(prompt.Index))
	for i, entry := range prompt.Index {
		slugs[i] = entry.Slug
	}
	return slugs
}

func promptTokens(prompt model.WikiPromptContext) int {
	total := 0
	for _, page := range prompt.Pages {
		total += wikiPromptPageTokens(page)
	}
	for _, entry := range prompt.Index {
		total += wikiPromptIndexTokens(entry)
	}
	return total
}

func TestBuildWikiPrompt_AlwaysPagesAreWholeAndOnDemandPagesAreOnlyInTheIndex(t *testing.T) {
	pages := []*model.WikiAgentPageRow{
		promptPage(1, testInstanceSpace, "security", constants.WikiAgentAccessAlways),
		promptPage(2, testProjectSpace, "how-we-work", constants.WikiAgentAccessAlways),
		promptPage(3, testProjectSpace, "runbook", constants.WikiAgentAccessOnDemand),
	}
	prompt, reads := buildWikiPrompt(wikiPromptInput{
		Pages:           withBodies(pages, map[int64]string{1: "no secrets in logs", 2: "tests first"}),
		IdsLinked:       map[int64]bool{},
		IdInstanceSpace: testInstanceSpace,
		TokenLimit:      1000,
	})

	require.Equal(t, []string{"how-we-work", "shared:security"}, promptSlugs(prompt))
	require.Equal(t, "tests first", prompt.Pages[0].Body)
	require.Equal(t, "always", prompt.Pages[0].Reason)
	require.Equal(t, 2, prompt.Pages[0].Version)
	require.Equal(t, []string{"runbook"}, indexSlugs(prompt))
	require.Equal(t, "about runbook", prompt.Index[0].Summary)
	require.Zero(t, prompt.IndexMore)

	sources := map[string]constants.WikiReadSource{}
	for _, read := range reads {
		sources[read.Page.Slug] = read.Source
		require.Positive(t, read.Tokens)
	}
	require.Equal(t, map[string]constants.WikiReadSource{
		"security":    constants.WikiReadPromptAlways,
		"how-we-work": constants.WikiReadPromptAlways,
		"runbook":     constants.WikiReadPromptIndex,
	}, sources)
}

func TestBuildWikiPrompt_LinkedOnDemandPageIsWholeAndComesFirst(t *testing.T) {
	pages := []*model.WikiAgentPageRow{
		promptPage(1, testProjectSpace, "conventions", constants.WikiAgentAccessAlways),
		promptPage(2, testProjectSpace, "gateway", constants.WikiAgentAccessOnDemand),
	}
	prompt, reads := buildWikiPrompt(wikiPromptInput{
		Pages:           withBodies(pages, map[int64]string{1: "rules", 2: "worktree per run"}),
		IdsLinked:       map[int64]bool{2: true},
		IdInstanceSpace: testInstanceSpace,
		TokenLimit:      1000,
	})

	require.Equal(t, []string{"gateway", "conventions"}, promptSlugs(prompt))
	require.Equal(t, "linked", prompt.Pages[0].Reason)
	require.Equal(t, "worktree per run", prompt.Pages[0].Body)
	require.Empty(t, prompt.Index)
	require.Equal(t, constants.WikiReadPromptLinked, reads[0].Source)
}

func TestBuildWikiPrompt_LinkedAlwaysPageIsListedOnceAsAlways(t *testing.T) {
	pages := []*model.WikiAgentPageRow{promptPage(1, testProjectSpace, "conventions", constants.WikiAgentAccessAlways)}
	prompt, reads := buildWikiPrompt(wikiPromptInput{
		Pages:           withBodies(pages, map[int64]string{1: "rules"}),
		IdsLinked:       map[int64]bool{1: true},
		IdInstanceSpace: testInstanceSpace,
		TokenLimit:      1000,
	})

	require.Len(t, prompt.Pages, 1)
	require.Equal(t, "always", prompt.Pages[0].Reason)
	require.Len(t, reads, 1)
	require.Equal(t, constants.WikiReadPromptAlways, reads[0].Source)
}

func TestBuildWikiPrompt_NeverExceedsTheTokenLimit(t *testing.T) {
	big := strings.Repeat("word ", 2000)
	pages := []*model.WikiAgentPageRow{
		promptPage(1, testProjectSpace, "linked-big", constants.WikiAgentAccessOnDemand),
		promptPage(2, testProjectSpace, "always-small", constants.WikiAgentAccessAlways),
		promptPage(3, testProjectSpace, "always-big", constants.WikiAgentAccessAlways),
	}
	for idPage := int64(10); idPage < 60; idPage++ {
		pages = append(pages, promptPage(idPage, testProjectSpace, "page-"+strings.Repeat("x", int(idPage%7)+1), constants.WikiAgentAccessOnDemand))
	}
	for _, limit := range []int{0, 5, 40, 300, 2600, 6000} {
		prompt, reads := buildWikiPrompt(wikiPromptInput{
			Pages:           withBodies(pages, map[int64]string{1: big, 2: "short", 3: big}),
			IdsLinked:       map[int64]bool{1: true},
			IdInstanceSpace: testInstanceSpace,
			TokenLimit:      limit,
		})
		require.LessOrEqual(t, promptTokens(prompt), limit, "limit %d", limit)
		total := 0
		for _, read := range reads {
			total += read.Tokens
		}
		require.Equal(t, promptTokens(prompt), total, "logged tokens match the prompt for limit %d", limit)
		require.Equal(t, len(pages), len(prompt.Pages)+len(prompt.Index)+prompt.IndexMore, "every page is accounted for at limit %d", limit)
	}
}

func TestBuildWikiPrompt_PageOverTheLimitMovesToTheTopOfTheIndex(t *testing.T) {
	pages := []*model.WikiAgentPageRow{
		promptPage(1, testProjectSpace, "other", constants.WikiAgentAccessOnDemand),
		promptPage(2, testProjectSpace, "huge", constants.WikiAgentAccessOnDemand),
		promptPage(3, testProjectSpace, "small", constants.WikiAgentAccessAlways),
	}
	prompt, _ := buildWikiPrompt(wikiPromptInput{
		Pages:           withBodies(pages, map[int64]string{2: strings.Repeat("a", 4000), 3: "fits"}),
		IdsLinked:       map[int64]bool{2: true},
		IdInstanceSpace: testInstanceSpace,
		TokenLimit:      200,
	})

	require.Equal(t, []string{"small"}, promptSlugs(prompt))
	require.Equal(t, []string{"huge", "other"}, indexSlugs(prompt))
	require.True(t, prompt.Index[0].OverLimit)
	require.False(t, prompt.Index[1].OverLimit)
}

func TestBuildWikiPrompt_CutsTheIndexAndCountsTheRest(t *testing.T) {
	pages := []*model.WikiAgentPageRow{}
	for idPage := int64(1); idPage <= 20; idPage++ {
		pages = append(pages, promptPage(idPage, testProjectSpace, "page", constants.WikiAgentAccessOnDemand))
	}
	entryCost := wikiPromptIndexTokens(model.WikiPromptIndexEntry{Slug: "page", Title: "PAGE", Summary: "about page"})
	prompt, _ := buildWikiPrompt(wikiPromptInput{
		Pages:           withBodies(pages, map[int64]string{}),
		IdsLinked:       map[int64]bool{},
		IdInstanceSpace: testInstanceSpace,
		TokenLimit:      entryCost*5 + 1,
	})

	require.Len(t, prompt.Index, 5)
	require.Equal(t, 15, prompt.IndexMore)
}

func TestBuildWikiPrompt_CapsAHugeProjectLimit(t *testing.T) {
	huge := strings.Repeat("b", (wikiPromptMaxTokens+100)*4)
	prompt, _ := buildWikiPrompt(wikiPromptInput{
		Pages:           withBodies([]*model.WikiAgentPageRow{promptPage(1, testProjectSpace, "huge", constants.WikiAgentAccessAlways)}, map[int64]string{1: huge}),
		IdsLinked:       map[int64]bool{},
		IdInstanceSpace: testInstanceSpace,
		TokenLimit:      200000,
	})

	require.Empty(t, prompt.Pages)
	require.Equal(t, []string{"huge"}, indexSlugs(prompt))
}

func TestBuildWikiPrompt_EmptyWikiGivesAnEmptyContext(t *testing.T) {
	prompt, reads := buildWikiPrompt(wikiPromptInput{IdInstanceSpace: testInstanceSpace, TokenLimit: 12000})

	require.Empty(t, prompt.Pages)
	require.Empty(t, prompt.Index)
	require.Empty(t, reads)
}

func TestBuildWikiPrompt_KeepsTheEncodedSizeUnderTheWebhookLimit(t *testing.T) {
	escaped := strings.Repeat("<&>", 40000)
	accented := strings.Repeat("ľščťžýáíé", 20000)
	pages := withBodies([]*model.WikiAgentPageRow{
		promptPage(1, testProjectSpace, "markup", constants.WikiAgentAccessAlways),
		promptPage(2, testProjectSpace, "accents", constants.WikiAgentAccessAlways),
		promptPage(3, testProjectSpace, "accents-again", constants.WikiAgentAccessAlways),
	}, map[int64]string{1: escaped, 2: accented, 3: accented})
	prompt, _ := buildWikiPrompt(wikiPromptInput{
		Pages:           pages,
		IdsLinked:       map[int64]bool{},
		IdInstanceSpace: testInstanceSpace,
		TokenLimit:      200000,
	})

	encoded, err := json.Marshal(prompt)
	require.NoError(t, err)
	require.LessOrEqual(t, len(encoded), wikiPromptMaxBytes+1000)
	require.NotContains(t, promptSlugs(prompt), "markup", "120 000 escaped characters are 720 kB of JSON")
	require.Contains(t, promptSlugs(prompt), "accents")
	require.NotContains(t, promptSlugs(prompt), "accents-again", "a second accented page no longer fits")
	require.Equal(t, []string{"markup", "accents-again"}, indexSlugs(prompt))
}
