package model

import "time"

type Release struct {
	IdRelease   int64      `json:"idRelease"    db:"id_release"`
	IdProject   int64      `json:"idProject"    db:"id_project"`
	IdTag       *int64     `json:"idTag"        db:"id_tag"`
	Name        string     `json:"name"         db:"name"`
	Status      string     `json:"status"       db:"status"`
	Notes       string     `json:"notes"        db:"notes"`
	RecreateTag bool       `json:"recreateTag"  db:"recreate_tag"`
	OpenIssues  string     `json:"openIssues"   db:"open_issues"`
	IssueCount  int        `json:"issueCount"   db:"issue_count"`
	CreateAt    time.Time  `json:"createAt"     db:"create_at"`
	CreateBy    *int64     `json:"createBy"     db:"create_by"`
	ReleasedAt  *time.Time `json:"releasedAt"   db:"released_at"`
	ReleasedBy  *int64     `json:"releasedBy"   db:"released_by"`
}

type ReleaseView struct {
	Release
	TagName        *string `json:"tagName"        db:"tag_name"`
	IdRunNotes     *int64  `json:"idRunNotes"     db:"id_run_notes"`
	NotesRunPhase  *string `json:"notesRunPhase"  db:"notes_run_phase"`
	NotesRunError  *string `json:"notesRunError"  db:"notes_run_error"`
	IsNotesPending bool    `json:"isNotesPending" db:"is_notes_pending"`
}

type ReleaseIssue struct {
	IdIssue        int64   `json:"idIssue"        db:"id_issue"`
	IdIssuePublic  int64   `json:"idIssuePublic"  db:"id_issue_public"`
	Title          string  `json:"title"          db:"title"`
	IdIssueType    *int64  `json:"idIssueType"    db:"id_issue_type"`
	IssueTypeName  *string `json:"issueTypeName"  db:"issue_type_name"`
	IssueTypeOrder *int    `json:"-"              db:"issue_type_order"`
	IsClosed       bool    `json:"-"              db:"is_closed"`
}

type ReleaseSection struct {
	IdIssueType *int64          `json:"idIssueType"`
	Name        *string         `json:"name"`
	Issues      []*ReleaseIssue `json:"issues"`
}

type ReleaseDraft struct {
	Sections   []*ReleaseSection `json:"sections"`
	OpenIssues []*ReleaseIssue   `json:"openIssues"`
}

type CreateReleaseReq struct {
	IdTag int64  `json:"idTag" binding:"required"`
	Name  string `json:"name"  binding:"required,max=40"`
}

type UpdateReleaseReq struct {
	Name        string  `json:"name"        binding:"required,max=40"`
	RecreateTag bool    `json:"recreateTag"`
	OpenIssues  string  `json:"openIssues"  binding:"required,oneof=move keep"`
	Notes       *string `json:"notes"`
}

type UpdateReleaseNotesReq struct {
	Notes string `json:"notes"`
}

type StartReleaseAgentReq struct {
	IdUserAgent int64 `json:"idUserAgent" binding:"required"`
	RunChoices
}

type DeleteReleaseReq struct {
	Tag         string `json:"tag"         binding:"omitempty,oneof=keep move_delete move_keep"`
	IdTagTarget *int64 `json:"idTagTarget"`
}

type ReleasePromptContext struct {
	Name       string            `json:"name"`
	Tag        string            `json:"tag"`
	Sections   []*ReleaseSection `json:"sections"`
	OpenIssues []*ReleaseIssue   `json:"openIssues"`
}
