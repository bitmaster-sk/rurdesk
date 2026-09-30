package model

import (
	"encoding/json"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
)

type CustomFieldOption struct {
	IdOption      int64  `json:"idOption"      db:"id_option"`
	IdCustomField int64  `json:"idCustomField" db:"id_custom_field"`
	Label         string `json:"label"         db:"label"`
	OrderRank     int    `json:"orderRank"     db:"order_rank"`
}

type CustomField struct {
	IdCustomField int64                     `json:"idCustomField" db:"id_custom_field"`
	IdProject     int64                     `json:"idProject"     db:"id_project"`
	Key           string                    `json:"key"           db:"key"`
	Name          string                    `json:"name"          db:"name"`
	FieldType     constants.CustomFieldType `json:"fieldType"     db:"field_type"`
	IsRequired    bool                      `json:"isRequired"    db:"is_required"`
	RequiredSince *time.Time                `json:"requiredSince" db:"required_since"`
	DefaultValue  json.RawMessage           `json:"defaultValue"  db:"default_value"`
	OrderRank     int                       `json:"orderRank"     db:"order_rank"`
	ArchivedAt    *time.Time                `json:"archivedAt"    db:"archived_at"`
	Options       []CustomFieldOption       `json:"options"       db:"-"`
}

func (f CustomField) IsArchived() bool { return f.ArchivedAt != nil }

func (f CustomField) HasOption(idOption int64) bool {
	for _, option := range f.Options {
		if option.IdOption == idOption {
			return true
		}
	}
	return false
}

type CustomFieldUsage struct {
	Issues        int           `json:"issues"`
	IssuesMissing int           `json:"issuesMissing"`
	OptionUsage   map[int64]int `json:"optionUsage"`
}
