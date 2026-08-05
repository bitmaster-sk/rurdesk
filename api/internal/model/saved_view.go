package model

import (
	"encoding/json"
	"time"
)

type SavedView struct {
	IdSavedView int64           `json:"idSavedView" db:"id_saved_view"`
	IdProject   int64           `json:"idProject"   db:"id_project"`
	Name        string          `json:"name"        db:"name"`
	ViewType    string          `json:"viewType"    db:"view_type"`
	Config      json.RawMessage `json:"config"      db:"config"`
	IsShared    bool            `json:"isShared"    db:"is_shared"`
	CreateBy    int64           `json:"createBy"    db:"create_by"`
	UpdateAt    time.Time       `json:"updateAt"    db:"update_at"`
}

type CreateSavedViewReq struct {
	// Trimmed before validation by the controller, so a name of blanks is a 400.
	Name     string          `json:"name"     binding:"required,max=60"`
	ViewType string          `json:"viewType" binding:"required,oneof=table kanban calendar gantt"`
	Config   json.RawMessage `json:"config"   binding:"required"`
	IsShared bool            `json:"isShared"`
}

type EditSavedViewReq struct {
	Name     string          `json:"name"     binding:"required,max=60"`
	ViewType string          `json:"viewType" binding:"required,oneof=table kanban calendar gantt"`
	Config   json.RawMessage `json:"config"   binding:"required"`
	IsShared bool            `json:"isShared"`
}
