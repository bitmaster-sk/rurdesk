package model

import (
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/google/uuid"
)

type AgentRunWikiRead struct {
	IdRead    int64                    `json:"idRead"    db:"id_read"`
	IdRun     int64                    `json:"idRun"     db:"id_run"`
	IdTask    *int64                   `json:"idTask"    db:"id_task"`
	Stage     string                   `json:"stage"     db:"stage"`
	IdCall    uuid.UUID                `json:"idCall"    db:"id_call"`
	Source    constants.WikiReadSource `json:"source"    db:"source"`
	IdPage    *int64                   `json:"idPage"    db:"id_page"`
	SpaceKind *constants.WikiSpaceKind `json:"spaceKind" db:"space_kind"`
	Slug      *string                  `json:"slug"      db:"slug"`
	Title     *string                  `json:"title"     db:"title"`
	VersionNo *int                     `json:"versionNo" db:"version_no"`
	Tokens    int                      `json:"tokens"    db:"tokens"`
	Query     *string                  `json:"query"     db:"query"`
	Pages     []WikiReadIndexPage      `json:"pages"     db:"pages"`
	CreateAt  time.Time                `json:"createAt"  db:"create_at"`
}

type WikiReadIndexPage struct {
	IdPage    int64                   `json:"idPage"`
	SpaceKind constants.WikiSpaceKind `json:"spaceKind"`
	Slug      string                  `json:"slug"`
	Title     string                  `json:"title"`
}

type WikiAgentPageRow struct {
	IdPage      int64                     `db:"id_page"`
	IdSpace     int64                     `db:"id_space"`
	Slug        string                    `db:"slug"`
	Title       string                    `db:"title"`
	Summary     string                    `db:"summary"`
	AgentAccess constants.WikiAgentAccess `db:"agent_access"`
	VersionNo   int                       `db:"version_no"`
	Body        *string                   `db:"body"`
}

type WikiPromptPage struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Version int    `json:"version"`
	Reason  string `json:"reason"`
	Body    string `json:"body"`
}

type WikiPromptIndexEntry struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	OverLimit bool   `json:"overLimit"`
}

type WikiPromptContext struct {
	Pages     []WikiPromptPage       `json:"pages"`
	Index     []WikiPromptIndexEntry `json:"index"`
	IndexMore int                    `json:"indexMore"`
}

type WikiAgentSearchHit struct {
	Slug    string                  `json:"slug"`
	Space   constants.WikiSpaceKind `json:"space"`
	Title   string                  `json:"title"`
	Summary string                  `json:"summary"`
	Snippet string                  `json:"snippet"`
	Version int                     `json:"version"`
}

type WikiAgentLink struct {
	Slug   string `json:"slug"`
	Title  string `json:"title"`
	Exists bool   `json:"exists"`
}

type WikiAgentPage struct {
	Slug          string                    `json:"slug"`
	Space         constants.WikiSpaceKind   `json:"space"`
	Title         string                    `json:"title"`
	Summary       string                    `json:"summary"`
	AgentAccess   constants.WikiAgentAccess `json:"agentAccess"`
	Version       int                       `json:"version"`
	LatestVersion int                       `json:"latestVersion"`
	Body          string                    `json:"body"`
	Links         []WikiAgentLink           `json:"links"`
	Issues        WikiPageIssueList         `json:"issues"`
}

type WikiAgentTreeNode struct {
	Slug        string                    `json:"slug"`
	Space       constants.WikiSpaceKind   `json:"space"`
	Title       string                    `json:"title"`
	Parent      *string                   `json:"parent"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess"`
}

type WikiAgentUpsertReq struct {
	Slug        string                    `json:"slug"`
	Space       constants.WikiSpaceKind   `json:"space"       binding:"omitempty,oneof=instance project"`
	Title       string                    `json:"title"       binding:"required,max=200"`
	Summary     *string                   `json:"summary"     binding:"omitempty,max=500"`
	Body        string                    `json:"body"`
	Parent      *string                   `json:"parent"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess" binding:"omitempty,oneof=always on_demand hidden"`
	BaseVersion int                       `json:"baseVersion" binding:"min=0"`
	Note        string                    `json:"note"        binding:"max=500"`
}

type WikiAgentUpsertRes struct {
	Slug       string `json:"slug"`
	Version    int    `json:"version"`
	Created    bool   `json:"created"`
	MergedFrom *int   `json:"mergedFrom"`
}
