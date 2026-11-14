package model

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/bitmaster-sk/rurdesk/api/internal/pointer"
	"github.com/bitmaster-sk/rurdesk/api/internal/timeutil"
)

const (
	IssueHistoryFieldState           = "state"
	IssueHistoryFieldSeverity        = "severity"
	IssueHistoryFieldIssueType       = "issue_type"
	IssueHistoryFieldAssignee        = "assignee"
	IssueHistoryFieldTitle           = "title"
	IssueHistoryFieldDescription     = "description"
	IssueHistoryFieldEstimated       = "estimated"
	IssueHistoryFieldScheduledAt     = "scheduled_at"
	IssueHistoryFieldPoints          = "points"
	IssueHistoryFieldTags            = "tags"
	IssueHistoryFieldIsSummary       = "is_summary"
	IssueHistoryFieldRollupEstimated = "rollup_estimated"
	IssueHistoryFieldRollupTracked   = "rollup_tracked"
)

const (
	IssueHistorySourceUser   = "user"
	IssueHistorySourceSystem = "system"
)

type IssueHistoryChange struct {
	Field    string
	OldValue any
	NewValue any
}

type IssueHistory struct {
	IdIssueHistory int64           `db:"id_issue_history"`
	IdIssue        int64           `db:"id_issue"`
	IdChange       uuid.UUID       `db:"id_change"`
	Field          string          `db:"field"`
	OldValue       json.RawMessage `db:"old_value"`
	NewValue       json.RawMessage `db:"new_value"`
	IdUser         *int64          `db:"id_user"`
	Source         string          `db:"source"`
	CreateAt       time.Time       `db:"create_at"`
}

type ReadIssueHistoryDto struct {
	IdIssueHistory int64           `json:"idIssueHistory"`
	IdChange       uuid.UUID       `json:"idChange"`
	Field          string          `json:"field"`
	OldValue       json.RawMessage `json:"oldValue"`
	NewValue       json.RawMessage `json:"newValue"`
	IdUser         *int64          `json:"idUser"`
	Source         string          `json:"source"`
	CreateAt       time.Time       `json:"createAt"`
}

type IssueHistoryUserDto struct {
	IdUser         int64   `json:"idUser"`
	Name           string  `json:"name"`
	ColorAvatarBg  string  `json:"colorAvatarBg"`
	AvatarInitials *string `json:"avatarInitials"`
	IsActive       bool    `json:"isActive"`
}

type ReadIssueHistoryRes struct {
	History []ReadIssueHistoryDto `json:"history"`
	Users   []IssueHistoryUserDto `json:"users"`
}

func NewIssueHistoryUserDto(user *User) IssueHistoryUserDto {
	return IssueHistoryUserDto{
		IdUser:         user.IdUser,
		Name:           user.Name,
		ColorAvatarBg:  user.ColorAvatarBg,
		AvatarInitials: user.AvatarInitials,
		IsActive:       user.IsActive,
	}
}

func IssueHistoryUserIds(history []IssueHistory) []int64 {
	var idsUser []int64
	add := func(idUser int64) {
		if !slices.Contains(idsUser, idUser) {
			idsUser = append(idsUser, idUser)
		}
	}
	for _, row := range history {
		if row.IdUser != nil {
			add(*row.IdUser)
		}
		if row.Field != IssueHistoryFieldAssignee {
			continue
		}
		for _, value := range []json.RawMessage{row.OldValue, row.NewValue} {
			var idAssignee *int64
			if err := json.Unmarshal(value, &idAssignee); err == nil && idAssignee != nil {
				add(*idAssignee)
			}
		}
	}
	return idsUser
}

func NewReadIssueHistoryDto(row IssueHistory) ReadIssueHistoryDto {
	return ReadIssueHistoryDto{
		IdIssueHistory: row.IdIssueHistory,
		IdChange:       row.IdChange,
		Field:          row.Field,
		OldValue:       jsonOrNull(row.OldValue),
		NewValue:       jsonOrNull(row.NewValue),
		IdUser:         row.IdUser,
		Source:         row.Source,
		CreateAt:       row.CreateAt,
	}
}

func jsonOrNull(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage("null")
	}
	return value
}

func DiffIssue(before, after *Issue) []IssueHistoryChange {
	var changes []IssueHistoryChange
	addInt64Ptr := func(field string, oldValue, newValue *int64) {
		if !pointer.Equal(oldValue, newValue) {
			changes = append(changes, IssueHistoryChange{Field: field, OldValue: pointer.Value(oldValue), NewValue: pointer.Value(newValue)})
		}
	}
	addInt64Ptr(IssueHistoryFieldState, before.IdState, after.IdState)
	addInt64Ptr(IssueHistoryFieldSeverity, before.IdSeverity, after.IdSeverity)
	addInt64Ptr(IssueHistoryFieldIssueType, before.IdIssueType, after.IdIssueType)
	addInt64Ptr(IssueHistoryFieldAssignee, before.AssignedTo, after.AssignedTo)
	if before.Title != after.Title {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldTitle, OldValue: before.Title, NewValue: after.Title})
	}
	if before.Description != after.Description {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldDescription, OldValue: before.Description, NewValue: after.Description})
	}
	if before.Estimated != after.Estimated {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldEstimated, OldValue: before.Estimated, NewValue: after.Estimated})
	}
	if !timeutil.PtrEqual(before.ScheduledAt, after.ScheduledAt) {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldScheduledAt, OldValue: pointer.Value(before.ScheduledAt), NewValue: pointer.Value(after.ScheduledAt)})
	}
	if !pointer.Equal(before.Points, after.Points) {
		changes = append(changes, IssueHistoryChange{Field: IssueHistoryFieldPoints, OldValue: pointer.Value(before.Points), NewValue: pointer.Value(after.Points)})
	}
	return append(changes, DiffTags(before.IdsTag, after.IdsTag)...)
}

func DiffTags(before, after []int64) []IssueHistoryChange {
	oldTags, newTags := sortedTags(before), sortedTags(after)
	if slices.Equal(oldTags, newTags) {
		return nil
	}
	return []IssueHistoryChange{{Field: IssueHistoryFieldTags, OldValue: oldTags, NewValue: newTags}}
}

func sortedTags(idsTag []int64) []int64 {
	sorted := slices.Clone(idsTag)
	if sorted == nil {
		sorted = []int64{}
	}
	slices.Sort(sorted)
	return sorted
}
