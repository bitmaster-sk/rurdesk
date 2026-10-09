package model

import (
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

type WikiSpace struct {
	IdSpace   int64                   `json:"idSpace"   db:"id_space"`
	Kind      constants.WikiSpaceKind `json:"kind"      db:"kind"`
	IdProject *int64                  `json:"idProject" db:"id_project"`
}

type WikiPage struct {
	IdPage        int64                     `json:"idPage"        db:"id_page"`
	IdSpace       int64                     `json:"idSpace"       db:"id_space"`
	IdParent      *int64                    `json:"idParent"      db:"id_parent"`
	Slug          string                    `json:"slug"          db:"slug"`
	Title         string                    `json:"title"         db:"title"`
	Summary       string                    `json:"summary"       db:"summary"`
	Body          string                    `json:"body"          db:"body"`
	AgentAccess   constants.WikiAgentAccess `json:"agentAccess"   db:"agent_access"`
	Rank          string                    `json:"rank"          db:"rank"`
	VersionNo     int                       `json:"versionNo"     db:"version_no"`
	DeletedAt     *time.Time                `json:"deletedAt"     db:"deleted_at"`
	DeletedBy     *int64                    `json:"deletedBy"     db:"deleted_by"`
	IdDeletedRoot *int64                    `json:"idDeletedRoot" db:"id_deleted_root"`
	CreateAt      time.Time                 `json:"createAt"      db:"create_at"`
	UpdateAt      time.Time                 `json:"updateAt"      db:"update_at"`
	CreateBy      *int64                    `json:"createBy"      db:"create_by"`
	UpdateBy      *int64                    `json:"updateBy"      db:"update_by"`
}

type WikiTreeNode struct {
	IdPage      int64                     `json:"idPage"      db:"id_page"`
	IdSpace     int64                     `json:"idSpace"     db:"id_space"`
	IdParent    *int64                    `json:"idParent"    db:"id_parent"`
	Slug        string                    `json:"slug"        db:"slug"`
	Title       string                    `json:"title"       db:"title"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess" db:"agent_access"`
	Rank        string                    `json:"rank"        db:"rank"`
}

type WikiPageVersion struct {
	IdVersion   int64                     `json:"idVersion"   db:"id_version"`
	IdPage      int64                     `json:"idPage"      db:"id_page"`
	VersionNo   int                       `json:"versionNo"   db:"version_no"`
	IdParent    *int64                    `json:"idParent"    db:"id_parent"`
	Title       string                    `json:"title"       db:"title"`
	Summary     string                    `json:"summary"     db:"summary"`
	Body        string                    `json:"body"        db:"body"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess" db:"agent_access"`
	Note        string                    `json:"note"        db:"note"`
	MergedFrom  *int                      `json:"mergedFrom"  db:"merged_from"`
	CreateAt    time.Time                 `json:"createAt"    db:"create_at"`
	CreateBy    *int64                    `json:"createBy"    db:"create_by"`
}

type WikiPageVersionSummary struct {
	VersionNo   int                       `json:"versionNo"   db:"version_no"`
	Title       string                    `json:"title"       db:"title"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess" db:"agent_access"`
	Note        string                    `json:"note"        db:"note"`
	MergedFrom  *int                      `json:"mergedFrom"  db:"merged_from"`
	CreateAt    time.Time                 `json:"createAt"    db:"create_at"`
	CreateBy    *int64                    `json:"createBy"    db:"create_by"`
}

type WikiPageDraft struct {
	IdPage      int64     `json:"idPage"      db:"id_page"`
	IdUser      int64     `json:"idUser"      db:"id_user"`
	BaseVersion int       `json:"baseVersion" db:"base_version"`
	Title       string    `json:"title"       db:"title"`
	Summary     string    `json:"summary"     db:"summary"`
	Body        string    `json:"body"        db:"body"`
	UpdateAt    time.Time `json:"updateAt"    db:"update_at"`
}

type WikiLink struct {
	IdSpaceTo  int64
	TargetSlug string
}

type WikiPageRef struct {
	IdPage  int64  `json:"idPage"  db:"id_page"`
	IdSpace int64  `json:"idSpace" db:"id_space"`
	Slug    string `json:"slug"    db:"slug"`
	Title   string `json:"title"   db:"title"`
}

type WikiIssuePage struct {
	IdIssue int64                         `json:"idIssue" db:"id_issue"`
	IdPage  int64                         `json:"idPage"  db:"id_page"`
	Source  constants.WikiIssueLinkSource `json:"source"  db:"source"`
}

type WikiTrashItem struct {
	IdPage      int64     `json:"idPage"      db:"id_page"`
	IdSpace     int64     `json:"idSpace"     db:"id_space"`
	IdParent    *int64    `json:"idParent"    db:"id_parent"`
	Title       string    `json:"title"       db:"title"`
	DeletedAt   time.Time `json:"deletedAt"   db:"deleted_at"`
	DeletedBy   *int64    `json:"deletedBy"   db:"deleted_by"`
	Descendants int       `json:"descendants" db:"descendants"`
	ParentAlive bool      `json:"parentAlive" db:"parent_alive"`
}

type WikiSearchHit struct {
	IdPage    int64   `json:"idPage"    db:"id_page"`
	IdSpace   int64   `json:"idSpace"   db:"id_space"`
	Slug      string  `json:"slug"      db:"slug"`
	Title     string  `json:"title"     db:"title"`
	Summary   string  `json:"summary"   db:"summary"`
	VersionNo int     `json:"versionNo" db:"version_no"`
	Snippet   string  `json:"snippet"   db:"snippet"`
	Score     float32 `json:"score"     db:"score"`
}

type CreateWikiPageReq struct {
	Space       constants.WikiSpaceKind   `json:"space"       binding:"required,oneof=instance project"`
	IdParent    *int64                    `json:"idParent"`
	Title       string                    `json:"title"       binding:"required,max=200"`
	Summary     string                    `json:"summary"     binding:"max=500"`
	Body        string                    `json:"body"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess" binding:"omitempty,oneof=always on_demand hidden"`
	Note        string                    `json:"note"        binding:"max=500"`
}

type SaveWikiPageReq struct {
	BaseVersion int                       `json:"baseVersion" binding:"required,min=1"`
	Title       string                    `json:"title"       binding:"required,max=200"`
	Summary     string                    `json:"summary"     binding:"max=500"`
	Body        string                    `json:"body"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess" binding:"required,oneof=always on_demand hidden"`
	Note        string                    `json:"note"        binding:"max=500"`
}

type MoveWikiPageReq struct {
	IdParent *int64 `json:"idParent"`
	IdPrev   *int64 `json:"idPrev"`
	IdNext   *int64 `json:"idNext"`
}

type WikiDraftReq struct {
	BaseVersion int    `json:"baseVersion" binding:"required,min=1"`
	Title       string `json:"title"       binding:"max=200"`
	Summary     string `json:"summary"     binding:"max=500"`
	Body        string `json:"body"`
}

type WikiMergePreviewReq struct {
	BaseVersion int                       `json:"baseVersion" binding:"required,min=1"`
	Title       string                    `json:"title"       binding:"max=200"`
	Summary     string                    `json:"summary"     binding:"max=500"`
	Body        string                    `json:"body"`
	AgentAccess constants.WikiAgentAccess `json:"agentAccess" binding:"omitempty,oneof=always on_demand hidden"`
}

type WikiHomeReq struct {
	IdPage *int64 `json:"idPage"`
}

type WikiSettingsReq struct {
	AlwaysTokenLimit int `json:"alwaysTokenLimit" binding:"min=0,max=200000"`
}

type WikiIssuePageReq struct {
	IdPage int64 `json:"idPage" binding:"required"`
}

type WikiSpaceView struct {
	IdSpace    int64                   `json:"idSpace"`
	Kind       constants.WikiSpaceKind `json:"kind"`
	CanEdit    bool                    `json:"canEdit"`
	CanManage  bool                    `json:"canManage"`
	IdHomePage *int64                  `json:"idHomePage"`
}

type WikiTree struct {
	Spaces        []WikiSpaceView `json:"spaces"`
	Nodes         []*WikiTreeNode `json:"nodes"`
	AlwaysTokens  int             `json:"alwaysTokens"`
	TokenLimit    int             `json:"tokenLimit"`
	TrashCount    int             `json:"trashCount"`
	ProposalCount int             `json:"proposalCount"`
}

type WikiResolvedLink struct {
	Shared  bool   `json:"shared"`
	Slug    string `json:"slug"`
	IdPage  *int64 `json:"idPage"`
	IdSpace int64  `json:"idSpace"`
	Title   string `json:"title"`
}

type WikiPageIssue struct {
	IdIssue       int64   `json:"idIssue"       db:"id_issue"`
	IdIssuePublic int64   `json:"idIssuePublic" db:"id_issue_public"`
	IdProject     int64   `json:"idProject"     db:"id_project"`
	Title         string  `json:"title"         db:"title"`
	StateName     *string `json:"stateName"     db:"state_name"`
	IsClosed      bool    `json:"isClosed"      db:"is_closed"`
}

type WikiPageIssueList struct {
	Items []WikiPageIssue `json:"items"`
	Total int             `json:"total"`
}

type WikiBacklinkList struct {
	Items []*WikiPageRef `json:"items"`
	Total int            `json:"total"`
}

type WikiPageView struct {
	Page      *WikiPage               `json:"page"`
	SpaceKind constants.WikiSpaceKind `json:"spaceKind"`
	CanEdit   bool                    `json:"canEdit"`
	CanManage bool                    `json:"canManage"`
	Backlinks WikiBacklinkList        `json:"backlinks"`
	Issues    WikiPageIssueList       `json:"issues"`
	Links     []WikiResolvedLink      `json:"links"`
	Draft     *WikiPageDraft          `json:"draft"`
}

type WikiIssueLink struct {
	IdPage    int64                         `json:"idPage"`
	IdSpace   int64                         `json:"idSpace"`
	SpaceKind constants.WikiSpaceKind       `json:"spaceKind"`
	Slug      string                        `json:"slug"`
	Title     string                        `json:"title"`
	Source    constants.WikiIssueLinkSource `json:"source"`
}

type WikiEditor struct {
	IdUser int64  `json:"idUser"`
	Name   string `json:"name"`
}

type WikiSavedNotice struct {
	IdPage    int64  `json:"idPage"`
	IdSpace   int64  `json:"idSpace"`
	VersionNo int    `json:"versionNo"`
	IdUser    int64  `json:"idUser"`
	UserName  string `json:"userName"`
}
