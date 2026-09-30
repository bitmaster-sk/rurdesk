package test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

func createIssueWithCustomFields(
	t *testing.T,
	app *issue.Application,
	token string,
	idProject int64,
	title string,
	customFields map[string]any,
) model.Issue {
	payload := map[string]any{
		"idProject":    idProject,
		"title":        title,
		"description":  "x",
		"customFields": customFields,
	}
	body, _ := json.Marshal(payload)
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created
}

func loadIssueCustomFields(t *testing.T, app *issue.Application, token string, idProject, idIssuePublic int64) map[string]any {
	res := Request(t, app, "GET",
		"/api/private/project/"+itoa(idProject)+"/issue/"+itoa(idIssuePublic), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var loaded model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&loaded))
	return loaded.CustomFields
}

func patchIssueCustomFields(t *testing.T, app *issue.Application, token string, idProject, idIssuePublic int64, customFields map[string]any) *http.Response {
	body, _ := json.Marshal(map[string]any{"customFields": customFields})
	return Request(t, app, "PATCH",
		"/api/private/project/"+itoa(idProject)+"/issue/"+itoa(idIssuePublic), string(body), token)
}

func TestIssueCustomFields_RoundTripAllTypes(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-roundtrip")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText})
	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact", FieldType: constants.CustomFieldTypeNumber})
	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "due", Name: "Due", FieldType: constants.CustomFieldTypeDate})
	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "reviewed", Name: "Reviewed", FieldType: constants.CustomFieldTypeBoolean})

	created := createIssueWithCustomFields(t, app, token, idProject, "rt", map[string]any{
		"note":     "hello",
		"impact":   7,
		"due":      "2026-10-12T00:00:00Z",
		"reviewed": true,
	})

	values := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)
	require.Equal(t, "hello", values["note"])
	require.Equal(t, float64(7), values["impact"])
	require.Equal(t, true, values["reviewed"])
	require.NotNil(t, values["due"])
}

func TestIssueCustomFields_PatchLeavesOtherKeysUntouched(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-partial")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText})
	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact", FieldType: constants.CustomFieldTypeNumber})

	created := createIssueWithCustomFields(t, app, token, idProject, "partial", map[string]any{
		"note": "keep me", "impact": 3,
	})

	res := patchIssueCustomFields(t, app, token, idProject, created.IdIssuePublic, map[string]any{"impact": 9})
	require.Equal(t, http.StatusOK, res.StatusCode)

	values := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)
	require.Equal(t, "keep me", values["note"])
	require.Equal(t, float64(9), values["impact"])
}

func TestIssueCustomFields_NullClearsValue(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-clear")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText})
	created := createIssueWithCustomFields(t, app, token, idProject, "clear", map[string]any{"note": "bye"})

	res := patchIssueCustomFields(t, app, token, idProject, created.IdIssuePublic, map[string]any{"note": nil})
	require.Equal(t, http.StatusOK, res.StatusCode)

	values := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)
	_, present := values["note"]
	require.False(t, present, "cleared key must be absent from the response map")
}

func TestIssueCustomFields_RejectsWrongTypeAndUnknownKey(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-reject")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact", FieldType: constants.CustomFieldTypeNumber})
	created := createIssueWithCustomFields(t, app, token, idProject, "reject", nil)

	res := patchIssueCustomFields(t, app, token, idProject, created.IdIssuePublic, map[string]any{"impact": "abc"})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	res = patchIssueCustomFields(t, app, token, idProject, created.IdIssuePublic, map[string]any{"nope": 1})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestIssueCustomFields_ArchivedFieldBlocksWriteButNotOtherEdits(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-archived")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "legacy", Name: "Legacy", FieldType: constants.CustomFieldTypeText})
	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText})
	created := createIssueWithCustomFields(t, app, token, idProject, "arch", map[string]any{"legacy": "old"})

	archive := `{"idProject":` + itoa(idProject) + `,"name":"Legacy","isRequired":false,"isArchived":true}`
	res := Request(t, app, "PATCH", "/api/private/custom-field/"+itoa(field.IdCustomField), archive, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = patchIssueCustomFields(t, app, token, idProject, created.IdIssuePublic, map[string]any{"legacy": "new"})
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	res = patchIssueCustomFields(t, app, token, idProject, created.IdIssuePublic, map[string]any{"note": "fine"})
	require.Equal(t, http.StatusOK, res.StatusCode)

	values := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)
	require.Equal(t, "old", values["legacy"], "archived values stay readable")
}

func createIssueRaw(t *testing.T, app *issue.Application, token string, idProject int64, payload string) *http.Response {
	return Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", payload, token)
}

func TestCreateIssue_RejectsMissingRequiredField(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-required-create")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact",
		FieldType: constants.CustomFieldTypeNumber, IsRequired: true})

	res := createIssueRaw(t, app, token, idProject,
		`{"idProject":`+itoa(idProject)+`,"title":"no impact","description":"x"}`)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	body, _ := io.ReadAll(res.Body)
	require.Contains(t, string(body), "impact", "the error must name the missing key so a caller can retry")

	res = createIssueRaw(t, app, token, idProject,
		`{"idProject":`+itoa(idProject)+`,"title":"no impact","description":"x","customFields":{"impact":null}}`)
	require.Equal(t, http.StatusBadRequest, res.StatusCode)

	created := createIssueWithCustomFields(t, app, token, idProject, "with impact", map[string]any{"impact": 7})
	values := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)
	require.Equal(t, float64(7), values["impact"])
}

func TestCreateIssue_IgnoresRequiredArchivedField(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-required-archived")

	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "legacy", Name: "Legacy",
		FieldType: constants.CustomFieldTypeText, IsRequired: true})

	archive := `{"idProject":` + itoa(idProject) + `,"name":"Legacy","isArchived":true}`
	res := Request(t, app, "PATCH", "/api/private/custom-field/"+itoa(field.IdCustomField), archive, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	res = createIssueRaw(t, app, token, idProject,
		`{"idProject":`+itoa(idProject)+`,"title":"archived required","description":"x"}`)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func TestEditIssue_LeavesRequiredFieldOfOlderIssueEmpty(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-required-old")

	created := createIssueWithCustomFields(t, app, token, idProject, "older", nil)

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact",
		FieldType: constants.CustomFieldTypeNumber, IsRequired: true})

	body := `{"title":"renamed after the rule"}`
	res := Request(t, app, "PATCH",
		"/api/private/project/"+itoa(idProject)+"/issue/"+itoa(created.IdIssuePublic), body, token)
	require.Equal(t, http.StatusOK, res.StatusCode, "an issue older than the rule must stay editable")

	values := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)
	_, present := values["impact"]
	require.False(t, present)
}

func TestSplitAccept_CopiesRequiredValuesToChildren(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-required-split")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact",
		FieldType: constants.CustomFieldTypeNumber, IsRequired: true})
	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText})

	parent := createIssueWithCustomFields(t, app, token, idProject, "to split",
		map[string]any{"impact": 3, "note": "not required"})

	body := `{"children":[{"title":"child one","description":"first"},{"title":"child two","description":"second"}]}`
	res := Request(t, app, "POST",
		"/api/private/project/"+itoa(idProject)+"/issue/"+itoa(parent.IdIssuePublic)+"/split/accept", body, token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	var accepted model.SplitAcceptRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&accepted))
	require.Len(t, accepted.Children, 2)

	for _, child := range accepted.Children {
		values := loadIssueCustomFields(t, app, token, idProject, child.IdIssuePublic)
		require.Equal(t, float64(3), values["impact"], "a split child must inherit required values")
		_, hasNote := values["note"]
		require.False(t, hasNote, "only required values are inherited")
	}
}

func TestCreateIssue_AppliesDefaultToOmittedField(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-default-create")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "owner", Name: "Owner",
		FieldType:    constants.CustomFieldTypeText,
		DefaultValue: json.RawMessage(`"unassigned"`)})

	created := createIssueWithCustomFields(t, app, token, idProject, "omitted", nil)
	require.Equal(t, "unassigned",
		loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)["owner"])

	answered := createIssueWithCustomFields(t, app, token, idProject, "answered", map[string]any{"owner": "Jane"})
	require.Equal(t, "Jane",
		loadIssueCustomFields(t, app, token, idProject, answered.IdIssuePublic)["owner"])
}

func TestCreateIssue_ExplicitNullKeepsAnOptionalFieldEmpty(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-default-null")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "owner", Name: "Owner",
		FieldType:    constants.CustomFieldTypeText,
		DefaultValue: json.RawMessage(`"unassigned"`)})

	created := createIssueWithCustomFields(t, app, token, idProject, "cleared", map[string]any{"owner": nil})
	_, present := loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)["owner"]
	require.False(t, present, "an explicit null means empty, which is not the same as unspecified")
}

func TestCreateIssue_DefaultSatisfiesARequiredField(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-default-required")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "impact", Name: "Impact",
		FieldType:    constants.CustomFieldTypeNumber,
		IsRequired:   true,
		DefaultValue: json.RawMessage(`5`)})

	created := createIssueWithCustomFields(t, app, token, idProject, "defaulted", nil)
	require.Equal(t, float64(5),
		loadIssueCustomFields(t, app, token, idProject, created.IdIssuePublic)["impact"])
}
