package test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadProjectSkills(t *testing.T, app *issue.Application, token string, idProject int64) []model.ProjectSkill {
	t.Helper()
	res := Request(t, app, "GET", "/api/private/project/"+itoa(idProject)+"/skills", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var entries []model.ProjectSkill
	require.Nil(t, json.NewDecoder(res.Body).Decode(&entries))
	return entries
}

func putProjectSkills(t *testing.T, app *issue.Application, token string, idProject int64, entries []model.UpdateProjectSkillReq) *http.Response {
	t.Helper()
	body, err := json.Marshal(entries)
	require.Nil(t, err)
	return Request(t, app, "PUT", "/api/private/project/"+itoa(idProject)+"/skills", string(body), token)
}

func TestProjectSkillMatrixReplaceAndLoad(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	seedBuiltinSkills(t)
	idProject := createProject(t, app, token, "skill-matrix-proj")

	all := listSkills(t, app, token)
	testingRules := skillByName(t, all, "Testing rules")
	prRules := skillByName(t, all, "PR rules")

	res := putProjectSkills(t, app, token, idProject, []model.UpdateProjectSkillReq{
		{IdSkill: testingRules.IdSkill, Stage: "implementation"},
		{IdSkill: prRules.IdSkill, Stage: "implementation"},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)

	entries := loadProjectSkills(t, app, token, idProject)
	require.Len(t, entries, 2, "PUT replaces the whole matrix")
	for _, entry := range entries {
		assert.Equal(t, "implementation", entry.Stage)
		assert.Equal(t, idProject, entry.IdProject)
	}
}

func TestProjectSkillMatrixSeedsDefaultsForNewProject(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	seedBuiltinSkills(t)
	idProject := createProject(t, app, token, "skill-defaults-proj")

	entries := loadProjectSkills(t, app, token, idProject)
	require.NotEmpty(t, entries, "builtins that ship enabled are on for a fresh project")

	all := listSkills(t, app, token)
	verificationRules := skillByName(t, all, "Verification rules")
	repositoryRules := skillByName(t, all, "Repository rules")
	testingRules := skillByName(t, all, "Testing rules")

	stagesOf := func(idSkill int64) []string {
		var stages []string
		for _, entry := range entries {
			if entry.IdSkill == idSkill {
				stages = append(stages, entry.Stage)
			}
		}
		return stages
	}

	assert.Equal(t, []string{"implementation"}, stagesOf(verificationRules.IdSkill))
	assert.ElementsMatch(t,
		[]string{"design", "implementation_plan", "implementation"},
		stagesOf(repositoryRules.IdSkill))
	assert.Empty(t, stagesOf(testingRules.IdSkill), "opinionated builtins ship switched off")
}

func TestProjectSkillMatrixRejectsUnknownSkill(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "skill-matrix-bad")

	res := putProjectSkills(t, app, token, idProject, []model.UpdateProjectSkillReq{
		{IdSkill: 999999, Stage: "design"},
	})
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestProjectSkillMatrixMemberCanReadButNotWrite(t *testing.T) {
	app := Setup(t)
	ownerToken := Token(t, app)
	seedBuiltinSkills(t)
	idProject := createProject(t, app, ownerToken, "skill-matrix-acl")

	memberToken := createUserAsAdmin(t, app, ownerToken,
		`{"name":"matrix-member","email":"matrix-member@test.sk","password":"kreslo1"}`)
	idMember := idOfUser(t, app, ownerToken, "matrix-member@test.sk")
	addBody, err := json.Marshal(model.AddProjectUserReq{IdUser: idMember, Role: model.RoleMember})
	require.Nil(t, err)
	addRes := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/member/user", string(addBody), ownerToken)
	require.Equal(t, http.StatusOK, addRes.StatusCode)

	readRes := Request(t, app, "GET", "/api/private/project/"+itoa(idProject)+"/skills", "", memberToken)
	assert.Equal(t, http.StatusOK, readRes.StatusCode)

	writeRes := putProjectSkills(t, app, memberToken, idProject, nil)
	assert.Equal(t, http.StatusForbidden, writeRes.StatusCode)
}

func TestProjectSkillMatrixDropsDeletedSkill(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "skill-matrix-cascade")

	create := Request(t, app, "POST", "/api/private/admin/skills",
		`{"name":"Cascade me","description":"d","content":"c"}`, token)
	require.Equal(t, http.StatusCreated, create.StatusCode)
	var custom model.Skill
	require.Nil(t, json.NewDecoder(create.Body).Decode(&custom))

	res := putProjectSkills(t, app, token, idProject, []model.UpdateProjectSkillReq{
		{IdSkill: custom.IdSkill, Stage: "design"},
	})
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Len(t, loadProjectSkills(t, app, token, idProject), 1)

	del := Request(t, app, "DELETE", "/api/private/admin/skills/"+itoa(custom.IdSkill), "", token)
	require.Equal(t, http.StatusNoContent, del.StatusCode)
	assert.Empty(t, loadProjectSkills(t, app, token, idProject), "deleting a skill cascades out of the matrix")
}
