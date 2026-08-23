package model

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
)

const (
	issueTitleMaxLen = 100
	issueMrIdMaxLen  = 50
)

type CreateIssueReq struct {
	IdProject      int64      `json:"idProject"`
	IdState        *int64     `json:"idState"      binding:"omitempty"`
	IdSeverity     *int64     `json:"idSeverity"   binding:"omitempty"`
	IdIssueType    *int64     `json:"idIssueType"  binding:"omitempty"`
	Title          string     `json:"title"        binding:"required,max=100"`
	Description    string     `json:"description"  binding:"required"`
	AssignedTo     *int64     `json:"assignedTo"   binding:"omitempty"`
	Estimated      int64      `json:"estimated"`
	Points         *int       `json:"points"       binding:"omitempty,min=0"`
	ScheduledAt    *time.Time `json:"scheduledAt"`
	IdempotencyKey *string    `json:"-"` // populated from Idempotency-Key header only
}

type EditIssueReq struct {
	IdProject        int64               `json:"idProject"`
	IdIssuePublic    int64               `json:"idIssuePublic"`
	IdState          Optional[int64]     `json:"idState,omitzero"`
	IdSeverity       Optional[int64]     `json:"idSeverity,omitzero"`
	IdIssueType      Optional[int64]     `json:"idIssueType,omitzero"`
	Title            Optional[string]    `json:"title,omitzero"`
	Description      Optional[string]    `json:"description,omitzero"`
	AssignedTo       Optional[int64]     `json:"assignedTo,omitzero"`
	Estimated        Optional[int64]     `json:"estimated,omitzero"`
	Points           Optional[int]       `json:"points,omitzero"`
	ScheduledAt      Optional[time.Time] `json:"scheduledAt,omitzero"`
	IdGitIntegration Optional[int64]     `json:"idGitIntegration,omitzero"`
	MrId             Optional[string]    `json:"mrId,omitzero"`
}

func (r *EditIssueReq) Validate() error {
	if r.Title.IsDefined {
		title := r.Title.OrElse("")
		if title == "" {
			return errs.ErrValidation.WithMessage("title is required")
		}
		if utf8.RuneCountInString(title) > issueTitleMaxLen {
			return errs.ErrValidation.WithMessage(fmt.Sprintf("title exceeds %d characters", issueTitleMaxLen))
		}
	}
	if r.Description.IsDefined && r.Description.OrElse("") == "" {
		return errs.ErrValidation.WithMessage("description is required")
	}
	if r.Points.IsDefined && r.Points.Value != nil && *r.Points.Value < 0 {
		return errs.ErrValidation.WithMessage("points must be zero or greater")
	}
	if r.MrId.IsDefined && r.MrId.Value != nil && utf8.RuneCountInString(*r.MrId.Value) > issueMrIdMaxLen {
		return errs.ErrValidation.WithMessage(fmt.Sprintf("mrId exceeds %d characters", issueMrIdMaxLen))
	}
	return nil
}

type BulkEditIssueEntryReq struct {
	IdIssuePublic  int64      `json:"idIssuePublic"  binding:"required"`
	ScheduledAt    *time.Time `json:"scheduledAt"`
	Estimated      *int64     `json:"estimated"`
	IdState        *int64     `json:"idState"`
	IdSeverity     *int64     `json:"idSeverity"`
	IdIssueType    *int64     `json:"idIssueType"`
	IdUserAssigned *int64     `json:"idUserAssigned"` // explicit null clears assignment
}

type BulkEditIssuesReq struct {
	Issues []BulkEditIssueEntryReq `json:"issues" binding:"required,min=1,max=100"`
}

type LoadIssuesReq struct {
	IdProject       int64     `json:"idProject"`
	OrderColumn     string    `json:"orderColumn"`
	OrderDirection  string    `json:"orderDirection"`
	IdsSeverity     []int64   `json:"idsSeverity"`
	SeverityUnset   bool      `json:"severityUnset"`
	IdsIssueType    []int64   `json:"idsIssueType"`
	IssueTypeUnset  bool      `json:"issueTypeUnset"`
	IdsState        []int64   `json:"idsState"`
	StateUnset      bool      `json:"stateUnset"`
	IdsAssignedTo   []int64   `json:"idsAssignedTo"`
	AssignedToUnset bool      `json:"assignedToUnset"`
	Title           string    `json:"title"`
	CreateAtFrom    time.Time `json:"createAtFrom"`
	CreateAtTo      time.Time `json:"createAtTo"`
	UpdateAtFrom    time.Time `json:"updateAtFrom"`
	UpdateAtTo      time.Time `json:"updateAtTo"`
	ScheduledAtFrom time.Time `json:"scheduledAtFrom"`
	ScheduledAtTo   time.Time `json:"scheduledAtTo"`
}

func (dto *LoadIssuesReq) GetOrder() *Order {
	return &Order{
		Column:    dto.OrderColumn,
		Direction: dto.OrderDirection,
	}
}

func NewLoadIssuesFilter(dto *LoadIssuesReq) *LoadIssuesFilter {
	return &LoadIssuesFilter{
		IdProject:       dto.IdProject,
		IdsSeverity:     dto.IdsSeverity,
		SeverityUnset:   dto.SeverityUnset,
		IdsIssueType:    dto.IdsIssueType,
		IssueTypeUnset:  dto.IssueTypeUnset,
		IdsState:        dto.IdsState,
		StateUnset:      dto.StateUnset,
		IdsAssignedTo:   dto.IdsAssignedTo,
		AssignedToUnset: dto.AssignedToUnset,
		Title:           dto.Title,
		CreateAtFrom:    dto.CreateAtFrom,
		CreateAtTo:      dto.CreateAtTo,
		UpdateAtFrom:    dto.UpdateAtFrom,
		UpdateAtTo:      dto.UpdateAtTo,
		ScheduledAtFrom: dto.ScheduledAtFrom,
		ScheduledAtTo:   dto.ScheduledAtTo,
		Order:           dto.GetOrder(),
	}
}
