package model

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
)

const customFieldTextMaxLen = 2000

type IssueCustomValue struct {
	IdIssue       int64                     `db:"id_issue"`
	IdCustomField int64                     `db:"id_custom_field"`
	FieldType     constants.CustomFieldType `db:"field_type"`
	ValueText     *string                   `db:"value_text"`
	ValueNumber   *float64                  `db:"value_number"`
	ValueDate     *time.Time                `db:"value_date"`
	ValueBool     *bool                     `db:"value_bool"`
	IdOption      *int64                    `db:"id_option"`
}

// Must dereference: the result goes into websocket payloads and HTTP responses,
// where a pointer would compare unequal to the value every consumer expects.
func (v IssueCustomValue) AsJson() any {
	switch v.FieldType {
	case constants.CustomFieldTypeText:
		if v.ValueText == nil {
			return nil
		}
		return *v.ValueText
	case constants.CustomFieldTypeNumber:
		if v.ValueNumber == nil {
			return nil
		}
		return *v.ValueNumber
	case constants.CustomFieldTypeDate:
		if v.ValueDate == nil {
			return nil
		}
		return *v.ValueDate
	case constants.CustomFieldTypeBoolean:
		if v.ValueBool == nil {
			return nil
		}
		return *v.ValueBool
	case constants.CustomFieldTypeSelect:
		if v.IdOption == nil {
			return nil
		}
		return *v.IdOption
	default:
		return nil
	}
}

func ParseCustomFieldValue(field CustomField, raw json.RawMessage) (IssueCustomValue, error) {
	value := IssueCustomValue{IdCustomField: field.IdCustomField, FieldType: field.FieldType}
	invalid := func() (IssueCustomValue, error) {
		return value, errs.ErrCustomFieldValueType.WithMessage(
			fmt.Sprintf("invalid value for field %q of type %s", field.Key, field.FieldType))
	}

	switch field.FieldType {
	case constants.CustomFieldTypeText:
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return invalid()
		}
		if utf8.RuneCountInString(text) > customFieldTextMaxLen {
			return value, errs.ErrCustomFieldValueType.WithMessage(
				fmt.Sprintf("field %q exceeds %d characters", field.Key, customFieldTextMaxLen))
		}
		value.ValueText = &text
	case constants.CustomFieldTypeNumber:
		var number float64
		if err := json.Unmarshal(raw, &number); err != nil {
			return invalid()
		}
		value.ValueNumber = &number
	case constants.CustomFieldTypeDate:
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return invalid()
		}
		parsed, err := time.Parse(time.RFC3339, text)
		if err != nil {
			return invalid()
		}
		value.ValueDate = &parsed
	case constants.CustomFieldTypeBoolean:
		var flag bool
		if err := json.Unmarshal(raw, &flag); err != nil {
			return invalid()
		}
		value.ValueBool = &flag
	case constants.CustomFieldTypeSelect:
		var idOption int64
		if err := json.Unmarshal(raw, &idOption); err != nil {
			return invalid()
		}
		if !field.HasOption(idOption) {
			return value, errs.ErrCustomFieldValueType.WithMessage(
				fmt.Sprintf("option %d does not belong to field %q", idOption, field.Key))
		}
		value.IdOption = &idOption
	default:
		return invalid()
	}
	return value, nil
}
