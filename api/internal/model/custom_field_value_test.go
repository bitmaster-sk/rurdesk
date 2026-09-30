package model

import (
	"encoding/json"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/stretchr/testify/require"
)

func numberField() CustomField {
	return CustomField{IdCustomField: 1, Key: "impact", Name: "Impact", FieldType: constants.CustomFieldTypeNumber}
}

func selectField() CustomField {
	return CustomField{
		IdCustomField: 2,
		Key:           "customer",
		Name:          "Customer",
		FieldType:     constants.CustomFieldTypeSelect,
		Options:       []CustomFieldOption{{IdOption: 10, Label: "Acme"}},
	}
}

func TestParseCustomFieldValue(t *testing.T) {
	tests := []struct {
		name    string
		field   CustomField
		raw     string
		wantErr bool
	}{
		{"number accepts integer", numberField(), `7`, false},
		{"number accepts decimal", numberField(), `7.5`, false},
		{"number rejects string", numberField(), `"abc"`, true},
		{"number rejects bool", numberField(), `true`, true},
		{"text accepts string", CustomField{IdCustomField: 3, FieldType: constants.CustomFieldTypeText}, `"hello"`, false},
		{"text rejects number", CustomField{IdCustomField: 3, FieldType: constants.CustomFieldTypeText}, `1`, true},
		{"date accepts rfc3339", CustomField{IdCustomField: 4, FieldType: constants.CustomFieldTypeDate}, `"2026-10-12T00:00:00Z"`, false},
		{"date rejects free text", CustomField{IdCustomField: 4, FieldType: constants.CustomFieldTypeDate}, `"tomorrow"`, true},
		{"boolean accepts true", CustomField{IdCustomField: 5, FieldType: constants.CustomFieldTypeBoolean}, `true`, false},
		{"boolean accepts false", CustomField{IdCustomField: 5, FieldType: constants.CustomFieldTypeBoolean}, `false`, false},
		{"boolean rejects string", CustomField{IdCustomField: 5, FieldType: constants.CustomFieldTypeBoolean}, `"yes"`, true},
		{"select accepts own option", selectField(), `10`, false},
		{"select rejects foreign option", selectField(), `99`, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCustomFieldValue(tc.field, json.RawMessage(tc.raw))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestParseCustomFieldValue_RejectsTextOverLimit(t *testing.T) {
	field := CustomField{IdCustomField: 3, FieldType: constants.CustomFieldTypeText}
	long := make([]rune, customFieldTextMaxLen+1)
	for i := range long {
		long[i] = 'a'
	}
	raw, _ := json.Marshal(string(long))

	_, err := ParseCustomFieldValue(field, raw)
	require.Error(t, err)
}
