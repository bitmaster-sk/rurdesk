package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/githost"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type GitIntegrationSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	UserID    int64
	ProjectID int64
}

func (s *GitIntegrationSuite) SetupSuite() {
	// Set a test encryption key so the controller can encrypt tokens.
	os.Setenv("GIT_INTEGRATION_ENCRYPTION_KEY", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	githost.ResetEncryptionKey()

	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	userRes := Request(s.T(), s.App, "GET", "/api/private/user", "", s.Token)
	var u model.User
	json.NewDecoder(userRes.Body).Decode(&u)
	s.UserID = u.IdUser

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"git-integration-test","color":"#aabbcc"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var prj model.Project
	json.NewDecoder(prjRes.Body).Decode(&prj)
	s.ProjectID = prj.IdProject
}

func (s *GitIntegrationSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.ProjectID)
}

func (s *GitIntegrationSuite) createIntegration(name, repoPath string) model.GitIntegrationRes {
	body := fmt.Sprintf(`{"name":%q,"hostType":"github","baseUrl":"https://github.com","repoPath":%q,"accessToken":"ghp_test_token"}`, name, repoPath)
	res := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID), body, s.Token)
	s.Require().Equal(http.StatusCreated, res.StatusCode)
	var integration model.GitIntegrationRes
	json.NewDecoder(res.Body).Decode(&integration)
	return integration
}

func (s *GitIntegrationSuite) Test_Create_AddsIntegration() {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID),
		`{"name":"api","hostType":"github","baseUrl":"https://github.com","repoPath":"org/api","accessToken":"ghp_token1"}`,
		s.Token)
	s.Require().Equal(http.StatusCreated, res.StatusCode)

	var body model.GitIntegrationRes
	json.NewDecoder(res.Body).Decode(&body)
	s.Greater(body.IdGitIntegration, int64(0))
	s.Equal("api", body.Name)
	s.Equal("github", body.HostType)
	s.True(body.HasToken)
}

func (s *GitIntegrationSuite) Test_Create_DuplicateRepo_409() {
	body := `{"name":"dup","hostType":"github","baseUrl":"https://github.com","repoPath":"org/dup","accessToken":"tok"}`
	res1 := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID), body, s.Token)
	s.Require().Equal(http.StatusCreated, res1.StatusCode)

	res2 := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID), body, s.Token)
	s.Equal(http.StatusConflict, res2.StatusCode)
}

func (s *GitIntegrationSuite) Test_Create_SameRepoOtherProject_OK() {
	// Create a second project.
	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"other-project","color":"#112233"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var otherPrj model.Project
	json.NewDecoder(prjRes.Body).Decode(&otherPrj)
	defer s.App.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE id_project = $1", otherPrj.IdProject)

	body := `{"name":"shared","hostType":"github","baseUrl":"https://github.com","repoPath":"org/shared","accessToken":"tok"}`
	res1 := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID), body, s.Token)
	s.Require().Equal(http.StatusCreated, res1.StatusCode)

	res2 := Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/git-integration", otherPrj.IdProject), body, s.Token)
	s.Equal(http.StatusCreated, res2.StatusCode)
}

func (s *GitIntegrationSuite) Test_List_ReturnsAllForProject_OrderedByCreatedAt() {
	for i := 0; i < 3; i++ {
		s.createIntegration(fmt.Sprintf("list-svc-%d", i), fmt.Sprintf("org/list-repo-%d", i))
	}

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var list []model.GitIntegrationRes
	json.NewDecoder(res.Body).Decode(&list)
	s.GreaterOrEqual(len(list), 3)
	for _, item := range list {
		s.True(item.HasToken)
	}
}

func (s *GitIntegrationSuite) Test_List_EmptyProject_ReturnsEmptyArray() {
	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"empty-git-prj","color":"#aabbcc"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var emptyPrj model.Project
	json.NewDecoder(prjRes.Body).Decode(&emptyPrj)
	defer s.App.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE id_project = $1", emptyPrj.IdProject)

	res := Request(s.T(), s.App, "GET", fmt.Sprintf("/api/private/project/%d/git-integration", emptyPrj.IdProject), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var list []model.GitIntegrationRes
	json.NewDecoder(res.Body).Decode(&list)
	s.Len(list, 0)
}

func (s *GitIntegrationSuite) Test_Get_ById_ReturnsRow() {
	created := s.createIntegration("get-by-id", "org/get-by-id")

	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", s.ProjectID, created.IdGitIntegration),
		"", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var body model.GitIntegrationRes
	json.NewDecoder(res.Body).Decode(&body)
	s.Equal(created.IdGitIntegration, body.IdGitIntegration)
}

func (s *GitIntegrationSuite) Test_Get_WrongProjectId_404() {
	created := s.createIntegration("cross-project", "org/cross")

	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"other-prj-404","color":"#ff0000"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var otherPrj model.Project
	json.NewDecoder(prjRes.Body).Decode(&otherPrj)
	defer s.App.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE id_project = $1", otherPrj.IdProject)

	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", otherPrj.IdProject, created.IdGitIntegration),
		"", s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func (s *GitIntegrationSuite) Test_Update_KeepsExistingTokenWhenAccessTokenOmitted() {
	created := s.createIntegration("keep-token", "org/keep-token")

	updateBody := fmt.Sprintf(`{"name":"keep-token-updated","hostType":"github","baseUrl":"https://github.com","repoPath":"org/keep-token"}`)
	res := Request(s.T(), s.App, "PUT",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", s.ProjectID, created.IdGitIntegration),
		updateBody, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	// Verify token is still set (hasToken=true) by reading the row.
	getRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", s.ProjectID, created.IdGitIntegration),
		"", s.Token)
	var updated model.GitIntegrationRes
	json.NewDecoder(getRes.Body).Decode(&updated)
	s.True(updated.HasToken)
	s.Equal("keep-token-updated", updated.Name)
}

func (s *GitIntegrationSuite) Test_Update_RotatesTokenWhenProvided() {
	created := s.createIntegration("rotate-token", "org/rotate-token")

	updateBody := fmt.Sprintf(`{"name":"rotate-token","hostType":"github","baseUrl":"https://github.com","repoPath":"org/rotate-token","accessToken":"ghp_new_token"}`)
	res := Request(s.T(), s.App, "PUT",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", s.ProjectID, created.IdGitIntegration),
		updateBody, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	s.True(res.StatusCode == http.StatusOK)
}

func (s *GitIntegrationSuite) Test_Delete_RemovesIntegrationAndUnlinksIssues() {
	created := s.createIntegration("delete-test", "org/delete-test")

	// Create an issue.
	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.ProjectID),
		`{"title":"delete-link-test","description":"test"}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)

	// Link issue to the integration.
	mrId := "42"
	linkBody := fmt.Sprintf(`{"title":"delete-link-test","description":"test","idGitIntegration":%d,"mrId":%q}`, created.IdGitIntegration, mrId)
	patchRes := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.ProjectID, iss.IdIssuePublic),
		linkBody, s.Token)
	s.Require().Equal(http.StatusOK, patchRes.StatusCode)

	// Delete integration.
	delRes := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", s.ProjectID, created.IdGitIntegration),
		"", s.Token)
	s.Require().Equal(http.StatusNoContent, delRes.StatusCode)

	// GET returns 404.
	getRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", s.ProjectID, created.IdGitIntegration),
		"", s.Token)
	s.Equal(http.StatusNotFound, getRes.StatusCode)

	// Issue still exists but link is cleared.
	issueGetRes := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.ProjectID, iss.IdIssuePublic),
		"", s.Token)
	s.Require().Equal(http.StatusOK, issueGetRes.StatusCode)
	var unlinked model.Issue
	json.NewDecoder(issueGetRes.Body).Decode(&unlinked)
	s.Nil(unlinked.IdGitIntegration)
	s.Nil(unlinked.MrId)
}

func (s *GitIntegrationSuite) Test_Create_InvalidHostType_400() {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID),
		`{"name":"bad","hostType":"bitbucket","baseUrl":"https://github.com","repoPath":"org/bad","accessToken":"tok"}`,
		s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *GitIntegrationSuite) Test_Create_ViewerForbidden() {
	// Register a viewer user.
	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"viewer-git","email":"viewer-git@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"viewer-git@test.sk","password":"kreslo"}`, "")
	var vTk struct{ Token string }
	json.NewDecoder(loginRes.Body).Decode(&vTk)

	viewerUserRes := Request(s.T(), s.App, "GET", "/api/private/user", "", vTk.Token)
	var viewerUser model.User
	json.NewDecoder(viewerUserRes.Body).Decode(&viewerUser)

	// Add as viewer to project.
	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID),
		fmt.Sprintf(`{"idUser":%d,"role":"viewer"}`, viewerUser.IdUser), s.Token)

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID),
		`{"name":"viewer-test","hostType":"github","baseUrl":"https://github.com","repoPath":"org/viewer","accessToken":"tok"}`,
		vTk.Token)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *GitIntegrationSuite) Test_Create_MemberForbidden() {
	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"member-git","email":"member-git@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"member-git@test.sk","password":"kreslo"}`, "")
	var mTk struct{ Token string }
	json.NewDecoder(loginRes.Body).Decode(&mTk)

	memberUserRes := Request(s.T(), s.App, "GET", "/api/private/user", "", mTk.Token)
	var memberUser model.User
	json.NewDecoder(memberUserRes.Body).Decode(&memberUser)

	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, memberUser.IdUser), s.Token)

	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID),
		`{"name":"member-test","hostType":"github","baseUrl":"https://github.com","repoPath":"org/member","accessToken":"tok"}`,
		mTk.Token)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *GitIntegrationSuite) Test_List_MemberAllowed() {
	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"member-list","email":"member-list@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"member-list@test.sk","password":"kreslo"}`, "")
	var mTk struct{ Token string }
	json.NewDecoder(loginRes.Body).Decode(&mTk)

	memberUserRes := Request(s.T(), s.App, "GET", "/api/private/user", "", mTk.Token)
	var memberUser model.User
	json.NewDecoder(memberUserRes.Body).Decode(&memberUser)

	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, memberUser.IdUser), s.Token)

	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID),
		"", mTk.Token)
	s.Equal(http.StatusOK, res.StatusCode)
}

func (s *GitIntegrationSuite) Test_Delete_MemberForbidden() {
	created := s.createIntegration("member-del", "org/member-del")

	Request(s.T(), s.App, "POST", "/api/private/admin/user",
		`{"name":"member-del","email":"member-del@test.sk","password":"kreslo"}`, Token(s.T(), s.App))
	loginRes := Request(s.T(), s.App, "POST", "/api/public/login",
		`{"email":"member-del@test.sk","password":"kreslo"}`, "")
	var mTk struct{ Token string }
	json.NewDecoder(loginRes.Body).Decode(&mTk)

	memberUserRes := Request(s.T(), s.App, "GET", "/api/private/user", "", mTk.Token)
	var memberUser model.User
	json.NewDecoder(memberUserRes.Body).Decode(&memberUser)

	Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectID),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, memberUser.IdUser), s.Token)

	res := Request(s.T(), s.App, "DELETE",
		fmt.Sprintf("/api/private/project/%d/git-integration/%d", s.ProjectID, created.IdGitIntegration),
		"", mTk.Token)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *GitIntegrationSuite) Test_TokenEncryption_Roundtrip() {
	const plainToken = "ghp_roundtrip_test_token_xyz"
	body := fmt.Sprintf(`{"name":"enc-test","hostType":"github","baseUrl":"https://github.com","repoPath":"org/enc-test","accessToken":%q}`, plainToken)
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/git-integration", s.ProjectID), body, s.Token)
	s.Require().Equal(http.StatusCreated, res.StatusCode)

	var created model.GitIntegrationRes
	json.NewDecoder(res.Body).Decode(&created)

	// Read raw bytes from DB and decrypt.
	var tokenEnc, nonce []byte
	err := s.App.Pool.QueryRow(context.Background(),
		"SELECT access_token_enc, token_nonce FROM projects.git_integration WHERE id_git_integration = $1",
		created.IdGitIntegration,
	).Scan(&tokenEnc, &nonce)
	s.Require().NoError(err)

	key, err := githost.LoadEncryptionKey()
	s.Require().NoError(err)

	decrypted, err := githost.Decrypt(key, nonce, tokenEnc)
	s.Require().NoError(err)
	s.Equal(plainToken, string(decrypted))
}

func (s *GitIntegrationSuite) Test_GetDiff_UnknownIntegration_404() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/git-integration/999999/mr/1/diff", s.ProjectID),
		"", s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func (s *GitIntegrationSuite) Test_GetStatus_UnknownIntegration_404() {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/project/%d/git-integration/999999/mr/1/status", s.ProjectID),
		"", s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func (s *GitIntegrationSuite) Test_Issue_PatchMrLink_BothOrNeither_400() {
	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.ProjectID),
		`{"title":"mr-link-test","description":"test"}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)

	// Set idGitIntegration but no mrId — must be 400.
	res1 := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.ProjectID, iss.IdIssuePublic),
		`{"title":"mr-link-test","description":"test","idGitIntegration":1}`,
		s.Token)
	s.Equal(http.StatusBadRequest, res1.StatusCode)

	// Set mrId but no idGitIntegration — must be 400.
	res2 := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", s.ProjectID, iss.IdIssuePublic),
		`{"title":"mr-link-test","description":"test","mrId":"42"}`,
		s.Token)
	s.Equal(http.StatusBadRequest, res2.StatusCode)
}

func (s *GitIntegrationSuite) Test_Issue_PatchMrLink_CrossProjectIntegration_404() {
	// Create integration under the test project.
	created := s.createIntegration("cross-link", "org/cross-link")

	// Create a different project.
	prjRes := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"cross-link-prj","color":"#001122"}`, s.Token)
	s.Require().Equal(http.StatusOK, prjRes.StatusCode)
	var otherPrj model.Project
	json.NewDecoder(prjRes.Body).Decode(&otherPrj)
	defer s.App.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE id_project = $1", otherPrj.IdProject)

	// Create issue in the other project.
	issueRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", otherPrj.IdProject),
		`{"title":"cross-link-issue","description":"test"}`, s.Token)
	s.Require().Equal(http.StatusOK, issueRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issueRes.Body).Decode(&iss)

	// Try to link the issue in otherPrj to the integration in s.ProjectID — must be 404.
	linkBody := fmt.Sprintf(`{"title":"cross-link-issue","description":"test","idGitIntegration":%d,"mrId":"1"}`, created.IdGitIntegration)
	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", otherPrj.IdProject, iss.IdIssuePublic),
		linkBody, s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func Test_RunGitIntegrationSuite(t *testing.T) {
	suite.Run(t, new(GitIntegrationSuite))
}
