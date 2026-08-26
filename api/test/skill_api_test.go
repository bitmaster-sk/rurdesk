package test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedBuiltinSkills(t *testing.T) {
	t.Helper()
	require.NoError(t, injector.GetSkillService().SyncBuiltins(context.Background()))
}

func listSkills(t *testing.T, app *issue.Application, token string) []model.Skill {
	t.Helper()
	res := Request(t, app, "GET", "/api/private/skills", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var all []model.Skill
	require.Nil(t, json.NewDecoder(res.Body).Decode(&all))
	return all
}

func skillByName(t *testing.T, all []model.Skill, name string) model.Skill {
	t.Helper()
	for _, s := range all {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("skill %q not in list", name)
	return model.Skill{}
}

func TestSkillApiListIsReadableByAnyUser(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	seedBuiltinSkills(t)
	userToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"plain","email":"plain-skill@test.sk","password":"kreslo1"}`)

	all := listSkills(t, app, userToken)
	require.NotEmpty(t, all)
	verificationRules := skillByName(t, all, "Verification rules")
	assert.True(t, verificationRules.IsBuiltin)
	assert.False(t, verificationRules.IsEdited)
	assert.NotEmpty(t, verificationRules.Description)
}

func TestSkillApiWritesAreAdminOnly(t *testing.T) {
	app := Setup(t)
	adminToken := Token(t, app)
	seedBuiltinSkills(t)
	userToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"plain2","email":"plain2-skill@test.sk","password":"kreslo1"}`)

	res := Request(t, app, "POST", "/api/private/admin/skills",
		`{"name":"Nope","description":"d","content":"c"}`, userToken)
	assert.Equal(t, http.StatusForbidden, res.StatusCode)
}

func TestSkillApiCrudLifecycle(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	seedBuiltinSkills(t)

	res := Request(t, app, "POST", "/api/private/admin/skills",
		`{"name":"Custom one","description":"d","content":"c"}`, token)
	require.Equal(t, http.StatusCreated, res.StatusCode)
	var created model.Skill
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	assert.False(t, created.IsBuiltin)
	assert.Equal(t, "c", created.Content)

	dup := Request(t, app, "POST", "/api/private/admin/skills",
		`{"name":"Custom one","description":"d","content":"c"}`, token)
	assert.Equal(t, http.StatusConflict, dup.StatusCode)

	del := Request(t, app, "DELETE", "/api/private/admin/skills/"+itoa(created.IdSkill), "", token)
	assert.Equal(t, http.StatusNoContent, del.StatusCode)

	gone := Request(t, app, "GET", "/api/private/admin/skills/"+itoa(created.IdSkill), "", token)
	assert.Equal(t, http.StatusNotFound, gone.StatusCode)
}

func TestSkillApiEditAndRestoreBuiltin(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	seedBuiltinSkills(t)

	builtin := skillByName(t, listSkills(t, app, token), "Verification rules")

	patch := Request(t, app, "PATCH", "/api/private/admin/skills/"+itoa(builtin.IdSkill),
		`{"content":"EDITED CONTENT"}`, token)
	require.Equal(t, http.StatusOK, patch.StatusCode)
	var patched model.Skill
	require.Nil(t, json.NewDecoder(patch.Body).Decode(&patched))
	assert.Equal(t, "EDITED CONTENT", patched.Content)
	assert.Equal(t, builtin.Name, patched.Name, "PATCH leaves omitted fields alone")
	assert.True(t, skillByName(t, listSkills(t, app, token), "Verification rules").IsEdited)

	restore := Request(t, app, "POST", "/api/private/admin/skills/"+itoa(builtin.IdSkill)+"/restore", "", token)
	require.Equal(t, http.StatusOK, restore.StatusCode)
	assert.False(t, skillByName(t, listSkills(t, app, token), "Verification rules").IsEdited)

	del := Request(t, app, "DELETE", "/api/private/admin/skills/"+itoa(builtin.IdSkill), "", token)
	assert.Equal(t, http.StatusConflict, del.StatusCode, "builtins cannot be deleted")
}

func TestSkillApiPatchTreatsNullAndOmittedAlikeButRejectsEmpty(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)

	create := Request(t, app, "POST", "/api/private/admin/skills",
		`{"name":"Patch subject","description":"d","content":"c"}`, token)
	require.Equal(t, http.StatusCreated, create.StatusCode)
	var created model.Skill
	require.Nil(t, json.NewDecoder(create.Body).Decode(&created))
	path := "/api/private/admin/skills/" + itoa(created.IdSkill)

	tests := []struct {
		name     string
		body     string
		expected int
	}{
		{"only the edited field is sent", `{"description":"only this"}`, http.StatusOK},
		{"an explicit null reads as untouched", `{"name":null,"description":"still fine"}`, http.StatusOK},
		{"an empty description is allowed", `{"description":""}`, http.StatusOK},
		{"an empty name is refused", `{"name":""}`, http.StatusBadRequest},
		{"an empty content is refused", `{"content":""}`, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := Request(t, app, "PATCH", path, tc.body, token)
			assert.Equal(t, tc.expected, res.StatusCode)
		})
	}

	after := Request(t, app, "GET", path, "", token)
	require.Equal(t, http.StatusOK, after.StatusCode)
	var final model.Skill
	require.Nil(t, json.NewDecoder(after.Body).Decode(&final))
	assert.Equal(t, "Patch subject", final.Name, "no accepted patch touched the name")
	assert.Equal(t, "c", final.Content, "no accepted patch touched the content")

	require.NoError(t, injector.GetSkillService().Delete(context.Background(), created.IdSkill))
}

func TestSkillApiRestoreRejectsCustomSkill(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)

	res := Request(t, app, "POST", "/api/private/admin/skills",
		`{"name":"Custom two","description":"d","content":"c"}`, token)
	require.Equal(t, http.StatusCreated, res.StatusCode)
	var created model.Skill
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))

	defer Request(t, app, "DELETE", "/api/private/admin/skills/"+itoa(created.IdSkill), "", token)

	restore := Request(t, app, "POST", "/api/private/admin/skills/"+itoa(created.IdSkill)+"/restore", "", token)
	assert.Equal(t, http.StatusConflict, restore.StatusCode)
}
