package model

type PinIssueView struct {
	IdIssue                 int64   `json:"idIssue"`
	IdIssuePublic           int64   `json:"idIssuePublic"`
	IdProject               int64   `json:"idProject"`
	IdSeverity              *int64  `json:"idSeverity"`
	Title                   string  `json:"title"`
	StateName               *string `json:"stateName"`
	StateIsStart            *bool   `json:"stateIsStart"`
	StateIsFinal            *bool   `json:"stateIsFinal"`
	AssignedToName          *string `json:"assignedToName"`
	AssignedToColorAvatarBg *string `json:"assignedToColorAvatarBg"`
}

type Pin struct {
	IdPin                int64         `json:"idPin" db:"id_pin"`
	IdIssue              int64         `json:"idIssue" db:"id_issue"`
	IdPinDestination     int64         `json:"idPinDestination" db:"id_pin_destination"`
	IdPinDestinationType int           `json:"idPinDestinationType" db:"id_pin_destination_type"`
	Issue                *PinIssueView `json:"issue"`
}

type LoadPinsReq struct {
	IdPinDestination     int64 `json:"idPinDestination"`
	IdPinDestinationType int   `json:"idPinDestinationType"`
}

type CreatePinReq struct {
	IdIssue              int64 `json:"idIssue" binding:"required"`
	IdPinDestination     int64 `json:"idPinDestination" binding:"required"`
	IdPinDestinationType int   `json:"idPinDestinationType" binding:"required"`
}

type PinDestinationType struct {
	IdPinDestinationType int    `json:"idPinDestinationType"`
	Code                 string `json:"code"`
}
