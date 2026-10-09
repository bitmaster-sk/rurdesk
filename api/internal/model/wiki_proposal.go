package model

import (
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

type WikiProposal struct {
	IdProposal      int64                          `json:"idProposal"      db:"id_proposal"`
	IdRun           int64                          `json:"idRun"           db:"id_run"`
	IdUserAgent     int64                          `json:"idUserAgent"     db:"id_user_agent"`
	RunPhase        string                         `json:"-"               db:"run_phase"`
	IdIssue         int64                          `json:"idIssue"         db:"id_issue"`
	IdIssuePublic   int64                          `json:"idIssuePublic"   db:"id_issue_public"`
	IssueTitle      string                         `json:"issueTitle"      db:"issue_title"`
	IdProject       int64                          `json:"idProject"       db:"id_project"`
	IdSpace         int64                          `json:"idSpace"         db:"id_space"`
	SpaceKind       constants.WikiSpaceKind        `json:"spaceKind"       db:"space_kind"`
	Kind            constants.WikiProposalKind     `json:"kind"            db:"kind"`
	IdPage          *int64                         `json:"idPage"          db:"id_page"`
	Slug            string                         `json:"slug"            db:"slug"`
	Title           string                         `json:"title"           db:"title"`
	Summary         string                         `json:"summary"         db:"summary"`
	Body            *string                        `json:"body"            db:"body"`
	IdParent        *int64                         `json:"idParent"        db:"id_parent"`
	ParentSlug      *string                        `json:"parentSlug"      db:"parent_slug"`
	ParentTitle     *string                        `json:"parentTitle"     db:"parent_title"`
	Reason          string                         `json:"reason"          db:"reason"`
	BaseVersion     *int                           `json:"baseVersion"     db:"base_version"`
	AgentAccess     *constants.WikiAgentAccess     `json:"agentAccess"     db:"agent_access"`
	Decision        constants.WikiProposalDecision `json:"-"               db:"decision"`
	State           constants.WikiProposalState    `json:"state"           db:"-"`
	DecidedBy       *int64                         `json:"decidedBy"       db:"decided_by"`
	DecidedAt       *time.Time                     `json:"decidedAt"       db:"decided_at"`
	DecisionNote    *string                        `json:"decisionNote"    db:"decision_note"`
	ResultVersion   *int                           `json:"resultVersion"   db:"result_version"`
	PageVersion     *int                           `json:"pageVersion"     db:"page_version"`
	PageTitle       *string                        `json:"pageTitle"       db:"page_title"`
	PageParentTitle *string                        `json:"pageParentTitle" db:"page_parent_title"`
	IsPageLive      bool                           `json:"isPageLive"      db:"is_page_live"`
	CreateAt        time.Time                      `json:"createAt"        db:"create_at"`
	UpdateAt        time.Time                      `json:"updateAt"        db:"update_at"`
}

type WikiProposalDraft struct {
	IdRun       int64
	IdIssue     int64
	IdProject   int64
	IdSpace     int64
	Kind        constants.WikiProposalKind
	IdPage      *int64
	Slug        string
	Title       string
	Summary     string
	Body        *string
	IdParent    *int64
	ParentSlug  *string
	Reason      string
	BaseVersion *int
}

type WikiProposalFilter struct {
	IdProject *int64
	IdIssue   *int64
	IdRun     *int64
	IdsSpace  []int64
	Decisions []constants.WikiProposalDecision
	OnlyLive  bool
}

type WikiProposalApproval struct {
	IdUser      int64
	Title       string
	Summary     string
	Body        *string
	BaseVersion *int
	AgentAccess *constants.WikiAgentAccess
	Note        *string
}

type WikiProposalSuggestReq struct {
	IdRun       int64                      `json:"idRun"`
	Kind        constants.WikiProposalKind `json:"kind"        binding:"required"`
	Slug        string                     `json:"slug"`
	Space       constants.WikiSpaceKind    `json:"space"`
	Title       string                     `json:"title"       binding:"max=200"`
	Summary     *string                    `json:"summary"     binding:"omitempty,max=500"`
	Body        *string                    `json:"body"`
	Parent      *string                    `json:"parent"`
	Reason      string                     `json:"reason"      binding:"required"`
	BaseVersion int                        `json:"baseVersion" binding:"min=0"`
}

type WikiProposalAgentView struct {
	Id          int64                       `json:"id"`
	Kind        constants.WikiProposalKind  `json:"kind"`
	Slug        string                      `json:"slug"`
	Title       string                      `json:"title"`
	Parent      *string                     `json:"parent,omitempty"`
	Reason      string                      `json:"reason"`
	BaseVersion *int                        `json:"baseVersion,omitempty"`
	State       constants.WikiProposalState `json:"state"`
	Updated     bool                        `json:"updated,omitempty"`
}

type WikiProposalAcceptReq struct {
	BaseVersion int                       `json:"baseVersion" binding:"min=0"`
	Title       *string                   `json:"title"       binding:"omitempty,max=200"`
	Summary     *string                   `json:"summary"     binding:"omitempty,max=500"`
	Body        *string                   `json:"body"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess" binding:"omitempty,oneof=always on_demand hidden"`
	Note        string                    `json:"note"        binding:"max=400"`
}

type WikiProposalRejectReq struct {
	Reason string `json:"reason" binding:"max=2000"`
}

type WikiProposalNotice struct {
	IdProject int64 `json:"idProject"`
	IdIssue   int64 `json:"idIssue"`
}
