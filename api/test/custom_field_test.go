package test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func createCustomField(t *testing.T, app *issue.Application, token string, req model.CreateCustomFieldReq) model.CustomField {
	body, _ := json.Marshal(req)
	res := Request(t, app, "POST", "/api/private/custom-field", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created
}

func loadCustomFields(t *testing.T, app *issue.Application, token string, idProject int64) []model.CustomField {
	res := Request(t, app, "GET", "/api/private/custom-field", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var all []model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&all))
	mine := make([]model.CustomField, 0, len(all))
	for _, field := range all {
		if field.IdProject == idProject {
			mine = append(mine, field)
		}
	}
	return mine
}

func createUserAndLogin(t *testing.T, app *issue.Application, adminToken, email string) string {
	res := Request(t, app, "POST", "/api/private/admin/user",
		`{"name":"stranger","email":"`+email+`","password":"kreslo"}`, adminToken)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "POST", "/api/public/login",
		`{"email":"`+email+`","password":"kreslo"}`, "")
	require.Equal(t, http.StatusOK, res.StatusCode)

	var token struct {
		Token string `json:"token"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&token))
	return token.Token
}

func TestCreateCustomField_AssignsOrderAndReturnsOptions(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-create")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject,
		Key:       "customer",
		Name:      "Zákazník",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}, {Label: "Northwind"}},
	})

	require.Equal(t, 1, field.OrderRank)
	require.Len(t, field.Options, 2)
	require.NotZero(t, field.Options[0].IdOption)
}

func TestCreateCustomField_RejectsDuplicateKey(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-dup-key")

	req := model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText,
	}
	createCustomField(t, app, token, req)

	body, _ := json.Marshal(req)
	res := Request(t, app, "POST", "/api/private/custom-field", string(body), token)
	require.Equal(t, http.StatusConflict, res.StatusCode)
}

func TestEditCustomField_RejectsKeyAndTypeChange(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-immutable")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText,
	})
	path := "/api/private/custom-field/" + itoa(field.IdCustomField)

	changeType := `{"idProject":` + itoa(idProject) + `,"name":"Note","fieldType":"number"}`
	res := Request(t, app, "PATCH", path, changeType, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	changeKey := `{"idProject":` + itoa(idProject) + `,"name":"Note","key":"other"}`
	res = Request(t, app, "PATCH", path, changeKey, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	renameOnly := `{"idProject":` + itoa(idProject) + `,"name":"Poznámka"}`
	res = Request(t, app, "PATCH", path, renameOnly, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var updated model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&updated))
	require.Equal(t, "Poznámka", updated.Name)
	require.Equal(t, "note", updated.Key)
	require.Equal(t, constants.CustomFieldTypeText, updated.FieldType)
}

func TestEditSelectCustomField_RenameKeepsOptions(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-rename-select")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}, {Label: "Northwind"}},
	})

	res := Request(t, app, "PATCH", "/api/private/custom-field/"+itoa(field.IdCustomField),
		`{"idProject":`+itoa(idProject)+`,"name":"Zákazník"}`, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var updated model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&updated))
	require.Equal(t, "Zákazník", updated.Name)
	require.Len(t, updated.Options, 2)
}

func TestEditCustomField_RenameDoesNotUnarchive(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-archive-sticky")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "legacy", Name: "Legacy", FieldType: constants.CustomFieldTypeText,
	})
	path := "/api/private/custom-field/" + itoa(field.IdCustomField)

	res := Request(t, app, "PATCH", path,
		`{"idProject":`+itoa(idProject)+`,"name":"Legacy","isArchived":true}`, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = Request(t, app, "PATCH", path,
		`{"idProject":`+itoa(idProject)+`,"name":"Legacy renamed"}`, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var updated model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&updated))
	require.NotNil(t, updated.ArchivedAt, "omitting isArchived must leave the field archived")
}

func TestReorderCustomFields_KeepsRanksDenseAndUnique(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-reorder")

	first := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "a", Name: "A", FieldType: constants.CustomFieldTypeText})
	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "b", Name: "B", FieldType: constants.CustomFieldTypeText})
	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "c", Name: "C", FieldType: constants.CustomFieldTypeText})

	res := Request(t, app, "PATCH", "/api/private/custom-field/"+itoa(first.IdCustomField),
		`{"idProject":`+itoa(idProject)+`,"name":"A","orderRank":3}`, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	fields := loadCustomFields(t, app, token, idProject)
	require.Equal(t, []string{"b", "c", "a"}, []string{fields[0].Key, fields[1].Key, fields[2].Key})
	require.Equal(t, []int{1, 2, 3}, []int{fields[0].OrderRank, fields[1].OrderRank, fields[2].OrderRank})
}

func TestCustomFieldWrites_ForbiddenForNonOwner(t *testing.T) {
	app := Setup(t)
	ownerToken := Token(t, app)
	idProject := createProject(t, app, ownerToken, "cf-acl")
	field := createCustomField(t, app, ownerToken, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText,
	})

	strangerToken := createUserAndLogin(t, app, ownerToken, "cf-stranger@test.sk")

	body, _ := json.Marshal(model.CreateCustomFieldReq{
		IdProject: idProject, Key: "sneaky", Name: "Sneaky", FieldType: constants.CustomFieldTypeText,
	})
	res := Request(t, app, "POST", "/api/private/custom-field", string(body), strangerToken)
	require.Equal(t, http.StatusForbidden, res.StatusCode)

	res = Request(t, app, "PATCH", "/api/private/custom-field/"+itoa(field.IdCustomField),
		`{"idProject":`+itoa(idProject)+`,"name":"Hacked"}`, strangerToken)
	require.Equal(t, http.StatusForbidden, res.StatusCode)

	res = Request(t, app, "DELETE",
		"/api/private/custom-field/"+itoa(field.IdCustomField)+"/project/"+itoa(idProject), "", strangerToken)
	require.Equal(t, http.StatusForbidden, res.StatusCode)
}

func TestDeleteCustomField_ConflictsWhileValuesExist(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-delete-conflict")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText,
	})
	createIssueWithCustomFields(t, app, token, idProject, "with value", map[string]any{"note": "x"})

	path := "/api/private/custom-field/" + itoa(field.IdCustomField) + "/project/" + itoa(idProject)
	res := Request(t, app, "DELETE", path, "", token)
	require.Equal(t, http.StatusConflict, res.StatusCode)

	res = Request(t, app, "DELETE", path+"?deleteValues=true", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestDeleteSelectCustomField_RemovesOptionsAndValues(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-delete-select")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}},
	})
	createIssueWithCustomFields(t, app, token, idProject, "sel",
		map[string]any{"customer": field.Options[0].IdOption})

	path := "/api/private/custom-field/" + itoa(field.IdCustomField) + "/project/" + itoa(idProject)
	res := Request(t, app, "DELETE", path+"?deleteValues=true", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var optionCount, valueCount int
	require.Nil(t, app.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM issues.custom_field_option WHERE id_custom_field = $1`,
		field.IdCustomField).Scan(&optionCount))
	require.Nil(t, app.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM issues.issue_custom_value WHERE id_custom_field = $1`,
		field.IdCustomField).Scan(&valueCount))
	require.Equal(t, 0, optionCount)
	require.Equal(t, 0, valueCount)
}

func TestCustomFieldUsage_CountsIssuesAndOptions(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-usage")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}},
	})
	idOption := field.Options[0].IdOption
	createIssueWithCustomFields(t, app, token, idProject, "u1", map[string]any{"customer": idOption})
	createIssueWithCustomFields(t, app, token, idProject, "u2", map[string]any{"customer": idOption})

	res := Request(t, app, "GET",
		"/api/private/custom-field/"+itoa(field.IdCustomField)+"/project/"+itoa(idProject)+"/usage", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var usage model.CustomFieldUsage
	require.Nil(t, json.NewDecoder(res.Body).Decode(&usage))
	require.Equal(t, 2, usage.Issues)
	require.Equal(t, 2, usage.OptionUsage[idOption])
}

func TestRemoveOption_RequiresIntentAndMigrates(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-option-intent")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}, {Label: "Northwind"}},
	})
	idAcme, idNorthwind := field.Options[0].IdOption, field.Options[1].IdOption
	created := createIssueWithCustomFields(t, app, token, idProject, "opt",
		map[string]any{"customer": idNorthwind})

	keepAcmeOnly := `{"idProject":` + itoa(idProject) + `,"name":"Customer","options":[{"idOption":` + itoa(idAcme) + `,"label":"Acme"}]}`
	path := "/api/private/custom-field/" + itoa(field.IdCustomField)

	res := Request(t, app, "PATCH", path, keepAcmeOnly, token)
	require.Equal(t, http.StatusConflict, res.StatusCode)

	res = Request(t, app, "PATCH", path+"?migrateOptionTo="+itoa(idAcme), keepAcmeOnly, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	values := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)
	require.Equal(t, float64(idAcme), values["customer"])
}

func patchCustomField(t *testing.T, app *issue.Application, token string, idCustomField int64, body string) *http.Response {
	return Request(t, app, "PATCH", "/api/private/custom-field/"+itoa(idCustomField), body, token)
}

func reloadCustomField(t *testing.T, app *issue.Application, token string, idProject, idCustomField int64) model.CustomField {
	for _, field := range loadCustomFields(t, app, token, idProject) {
		if field.IdCustomField == idCustomField {
			return field
		}
	}
	require.FailNow(t, "custom field disappeared")
	return model.CustomField{}
}

func TestCustomField_RequiredSinceFollowsTheFlag(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-required-since")

	optional := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText})
	require.Nil(t, optional.RequiredSince)

	required := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact",
		FieldType: constants.CustomFieldTypeNumber, IsRequired: true})
	require.NotNil(t, required.RequiredSince)

	res := patchCustomField(t, app, token, optional.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Note","isRequired":true}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var turnedOn model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&turnedOn))
	require.NotNil(t, turnedOn.RequiredSince)

	res = patchCustomField(t, app, token, optional.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Note renamed"}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var renamed model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&renamed))
	require.True(t, renamed.IsRequired, "an omitted isRequired must not clear the flag")
	require.NotNil(t, renamed.RequiredSince)
	require.Equal(t, turnedOn.RequiredSince.Unix(), renamed.RequiredSince.Unix(),
		"a rename must not move the moment the rule started")

	res = patchCustomField(t, app, token, optional.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Note renamed","isRequired":false}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var turnedOff model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&turnedOff))
	require.Nil(t, turnedOff.RequiredSince)
}

func TestCustomField_ArchiveAndRestoreKeepRequired(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-archive-required")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact",
		FieldType: constants.CustomFieldTypeNumber, IsRequired: true})

	res := patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Impact","isArchived":true}`)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Impact","isArchived":false}`)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var restored model.CustomField
	require.Nil(t, json.NewDecoder(res.Body).Decode(&restored))
	require.True(t, restored.IsRequired, "archiving must not silently demote a required field")
	require.NotNil(t, restored.RequiredSince)
}

func TestEditCustomField_RejectsOptionOfAnotherField(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-foreign-option")
	idOther := createProject(t, app, token, "cf-foreign-option-other")

	mine := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}},
	})
	theirs := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idOther, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Northwind"}},
	})
	idTheirOption := theirs.Options[0].IdOption

	body := `{"idProject":` + itoa(idProject) + `,"name":"Customer","options":[{"idOption":` +
		itoa(idTheirOption) + `,"label":"Hijacked"}]}`
	res := patchCustomField(t, app, token, mine.IdCustomField, body)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	untouched := loadCustomFields(t, app, token, idOther)
	require.Len(t, untouched, 1)
	require.Len(t, untouched[0].Options, 1)
	require.Equal(t, "Northwind", untouched[0].Options[0].Label)
}

func TestEditSelectCustomField_RejectsEmptyOptionList(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-empty-options")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}},
	})

	res := patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Customer","options":[]}`)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestRemoveOption_RequiredFieldRefusesToDropValues(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-required-option-drop")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect, IsRequired: true,
		Options: []model.CustomFieldOptionReq{{Label: "Acme"}, {Label: "Northwind"}},
	})
	idAcme, idNorthwind := field.Options[0].IdOption, field.Options[1].IdOption
	created := createIssueWithCustomFields(t, app, token, idProject, "opt",
		map[string]any{"customer": idNorthwind})

	keepAcmeOnly := `{"idProject":` + itoa(idProject) + `,"name":"Customer","options":[{"idOption":` +
		itoa(idAcme) + `,"label":"Acme"}]}`

	res := Request(t, app, "PATCH",
		"/api/private/custom-field/"+itoa(field.IdCustomField)+"?deleteOptionValues=true", keepAcmeOnly, token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	values := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)
	require.Equal(t, float64(idNorthwind), values["customer"], "the value must survive a refused drop")
}

func TestCustomFieldUsage_CountsIssuesWithoutValue(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-missing-count")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact", FieldType: constants.CustomFieldTypeNumber})
	createIssueWithCustomFields(t, app, token, idProject, "filled", map[string]any{"impact": 1})
	createIssueWithCustomFields(t, app, token, idProject, "empty one", nil)
	createIssueWithCustomFields(t, app, token, idProject, "empty two", nil)

	res := Request(t, app, "GET",
		"/api/private/custom-field/"+itoa(field.IdCustomField)+"/project/"+itoa(idProject)+"/usage", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var usage model.CustomFieldUsage
	require.Nil(t, json.NewDecoder(res.Body).Decode(&usage))
	require.Equal(t, 1, usage.Issues)
	require.Equal(t, 2, usage.IssuesMissing)
}

func TestCustomFieldBackfill_FillsOnlyIssuesWithoutValue(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-backfill")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact", FieldType: constants.CustomFieldTypeNumber})
	answered := createIssueWithCustomFields(t, app, token, idProject, "answered", map[string]any{"impact": 9})
	blank := createIssueWithCustomFields(t, app, token, idProject, "blank", nil)

	res := patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Impact","isRequired":true,"backfill":1}`)
	require.Equal(t, http.StatusOK, res.StatusCode)

	filled := loadIssueCustomFields(t, app, token, idProject, blank.IdIssuePublic)
	require.Equal(t, float64(1), filled["impact"])

	kept := loadIssueCustomFields(t, app, token, idProject, answered.IdIssuePublic)
	require.Equal(t, float64(9), kept["impact"], "a bulk fill must never overwrite an existing answer")
}

func TestCustomFieldUpdate_WithoutBackfillLeavesIssuesEmpty(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-no-backfill")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact", FieldType: constants.CustomFieldTypeNumber})
	blank := createIssueWithCustomFields(t, app, token, idProject, "blank", nil)

	res := patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Impact","isRequired":true}`)
	require.Equal(t, http.StatusOK, res.StatusCode)

	values := loadIssueCustomFields(t, app, token, idProject, blank.IdIssuePublic)
	_, present := values["impact"]
	require.False(t, present, "filling existing issues is the user's choice, not a side effect")
}

func TestCustomFieldDefault_SetOnCreateAndCleared(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-default-lifecycle")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "owner", Name: "Owner",
		FieldType:    constants.CustomFieldTypeText,
		DefaultValue: json.RawMessage(`"unassigned"`)})
	require.JSONEq(t, `"unassigned"`, string(field.DefaultValue))

	res := patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Owner"}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.JSONEq(t, `"unassigned"`, string(reloadCustomField(t, app, token, idProject, field.IdCustomField).DefaultValue),
		"a rename must not touch the default")

	res = patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Owner","defaultValue":null}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.JSONEq(t, "null", string(reloadCustomField(t, app, token, idProject, field.IdCustomField).DefaultValue))
}

func TestCustomFieldDefault_RejectsValueOfAnotherType(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-default-type")

	body, _ := json.Marshal(model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact",
		FieldType:    constants.CustomFieldTypeNumber,
		DefaultValue: json.RawMessage(`"high"`)})
	res := Request(t, app, "POST", "/api/private/custom-field", string(body), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestCustomFieldDefault_SelectPointsAtAnExistingOption(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-default-select")

	body, _ := json.Marshal(model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType:    constants.CustomFieldTypeSelect,
		Options:      []model.CustomFieldOptionReq{{Label: "Acme"}},
		DefaultValue: json.RawMessage(`1`)})
	res := Request(t, app, "POST", "/api/private/custom-field", string(body), token)
	require.Equal(t, http.StatusBadRequest, res.StatusCode, "options have no ids before they are inserted")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}, {Label: "Northwind"}}})

	res = patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Customer","defaultValue":999999}`)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	acme := field.Options[0]
	res = patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Customer","defaultValue":`+itoa(acme.IdOption)+`}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.JSONEq(t, itoa(acme.IdOption),
		string(reloadCustomField(t, app, token, idProject, field.IdCustomField).DefaultValue))
}

func TestCustomFieldDefault_DroppedWithTheOptionItPointedAt(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-default-option-gone")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "customer", Name: "Customer",
		FieldType: constants.CustomFieldTypeSelect,
		Options:   []model.CustomFieldOptionReq{{Label: "Acme"}, {Label: "Northwind"}}})
	acme, northwind := field.Options[0], field.Options[1]

	res := patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Customer","defaultValue":`+itoa(acme.IdOption)+`}`)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = patchCustomField(t, app, token, field.IdCustomField,
		`{"idProject":`+itoa(idProject)+`,"name":"Customer","options":[{"idOption":`+itoa(northwind.IdOption)+`,"label":"Northwind"}]}`)
	require.Equal(t, http.StatusOK, res.StatusCode)

	require.JSONEq(t, "null", string(reloadCustomField(t, app, token, idProject, field.IdCustomField).DefaultValue),
		"a default pointing at a deleted option would reject every new issue")

	created := createIssueWithCustomFields(t, app, token, idProject, "after the option went", nil)
	require.NotZero(t, created.IdIssue)
}
