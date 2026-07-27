package test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/stretchr/testify/suite"
)

// VersionSuite verifies the build-identity endpoint: admins can read it,
// everyone else cannot (the version of a public instance is not something to
// hand out freely).
type VersionSuite struct {
	suite.Suite
	App        *issue.Application
	AdminToken string
}

func (s *VersionSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.AdminToken = Token(s.T(), s.App)
}

func (s *VersionSuite) Test_01_Admin_ReadsBuildIdentity() {
	res := Request(s.T(), s.App, "GET", "/api/private/admin/version", "", s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var body struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&body))
	// The test binary is built without -ldflags, so it reports the dev defaults.
	s.Equal("dev", body.Version)
	s.Equal("unknown", body.Commit)
}

func (s *VersionSuite) Test_02_NonAdmin_Forbidden() {
	member := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"versionmember","email":"versionmember@test.sk","password":"kreslo"}`)
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email = 'versionmember@test.sk'")

	res := Request(s.T(), s.App, "GET", "/api/private/admin/version", "", member)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *VersionSuite) Test_03_Anonymous_Unauthorized() {
	res := Request(s.T(), s.App, "GET", "/api/private/admin/version", "", "")
	s.Equal(http.StatusUnauthorized, res.StatusCode)
}

func Test_RunVersionSuite(t *testing.T) {
	suite.Run(t, new(VersionSuite))
}
