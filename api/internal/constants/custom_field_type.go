package constants

type CustomFieldType string

const (
	CustomFieldTypeText    CustomFieldType = "text"
	CustomFieldTypeNumber  CustomFieldType = "number"
	CustomFieldTypeDate    CustomFieldType = "date"
	CustomFieldTypeSelect  CustomFieldType = "select"
	CustomFieldTypeBoolean CustomFieldType = "boolean"
)

func IsValidCustomFieldType(fieldType CustomFieldType) bool {
	switch fieldType {
	case CustomFieldTypeText, CustomFieldTypeNumber, CustomFieldTypeDate,
		CustomFieldTypeSelect, CustomFieldTypeBoolean:
		return true
	default:
		return false
	}
}
