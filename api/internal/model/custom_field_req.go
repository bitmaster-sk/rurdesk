package model

import (
	"encoding/json"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

type CustomFieldOptionReq struct {
	IdOption int64  `json:"idOption"`
	Label    string `json:"label" binding:"required,max=60"`
}

type CreateCustomFieldReq struct {
	IdProject  int64                     `json:"idProject"`
	Key        string                    `json:"key"        binding:"required,max=40"`
	Name       string                    `json:"name"       binding:"required,max=60"`
	FieldType  constants.CustomFieldType `json:"fieldType"  binding:"required"`
	IsRequired bool                      `json:"isRequired"`
	Options    []CustomFieldOptionReq    `json:"options"`
	Backfill   json.RawMessage           `json:"backfill,omitempty"`
	// A select field cannot carry a default on create: its options have no ids
	// before they are inserted.
	DefaultValue json.RawMessage `json:"defaultValue,omitempty"`
}

// Key and FieldType are immutable but still travel on the request: without them a
// change attempt is indistinguishable from an omission and cannot be rejected.
type EditCustomFieldReq struct {
	IdCustomField int64                     `json:"idCustomField"`
	IdProject     int64                     `json:"idProject"`
	Name          string                    `json:"name"       binding:"required,max=60"`
	Key           string                    `json:"key"`
	FieldType     constants.CustomFieldType `json:"fieldType"`
	IsRequired    Optional[bool]            `json:"isRequired,omitzero"`
	OrderRank     int                       `json:"orderRank"`
	IsArchived    Optional[bool]            `json:"isArchived,omitzero"`
	Options       []CustomFieldOptionReq    `json:"options"`
	// Never overwrites an existing value, so a caller cannot destroy answers by
	// resending it.
	Backfill json.RawMessage `json:"backfill,omitempty"`
	// DefaultValue is tri-state: absent leaves the stored default alone, JSON null
	// removes it, anything else replaces it.
	DefaultValue json.RawMessage `json:"defaultValue,omitempty"`
}
