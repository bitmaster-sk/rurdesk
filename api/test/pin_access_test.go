package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// PinAccessSuite covers: GetPins must not expose pinned issues the viewer cannot read.
// Issue visibility is project-scoped, so a pin may reference an issue in a project the viewer
// has no access to (pinned to their own user-page, or to a shared project-page).
type PinAccessSuite struct {
	suite.Suite
	App             *issue.Application
	OToken          string // owner of both projects
	CToken          string // member of project A only
	UToken          string // no project access
	CID             int64
	UID             int64
	ProjectA        int64
	ProjectB        int64
	IssueA          int64 // internal id_issue, project A
	IssueB          int64 // internal id_issue, project B
	UserPageType    int
	ProjectPageType int
}

func (s *PinAccessSuite) SetupSuite() {
	s.App = Setup(s.T())
	adminToken := Token(s.T(), s.App)

	s.OToken = createUserAsAdmin(s.T(), s.App, adminToken,
		`{"name":"pinowner","email":"pinowner@test.sk","password":"kreslo"}`)
	s.CToken = createUserAsAdmin(s.T(), s.App, adminToken,
		`{"name":"pinc","email":"pinc@test.sk","password":"kreslo"}`)
	s.UToken = createUserAsAdmin(s.T(), s.App, adminToken,
		`{"name":"pinu","email":"pinu@test.sk","password":"kreslo"}`)
	s.CID = idOfUser(s.T(), s.App, adminToken, "pinc@test.sk")
	s.UID = idOfUser(s.T(), s.App, adminToken, "pinu@test.sk")

	s.ProjectA = createProject(s.T(), s.App, s.OToken, "pin-access-project-a")
	s.ProjectB = createProject(s.T(), s.App, s.OToken, "pin-access-project-b")

	// C is a member of A only (not B).
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", s.ProjectA),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.CID), s.OToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	s.IssueA = s.createIssue(s.ProjectA, "Issue A")
	s.IssueB = s.createIssue(s.ProjectB, "Issue B")

	// Resolve pin destination type ids by code.
	dtRes := Request(s.T(), s.App, "GET", "/api/private/pin/destination-type", "", s.OToken)
	s.Require().Equal(http.StatusOK, dtRes.StatusCode)
	var types []model.PinDestinationType
	s.Require().Nil(json.NewDecoder(dtRes.Body).Decode(&types))
	for _, t := range types {
		switch t.Code {
		case "user-page":
			s.UserPageType = t.IdPinDestinationType
		case "project-page":
			s.ProjectPageType = t.IdPinDestinationType
		}
	}
	s.Require().NotZero(s.UserPageType)
	s.Require().NotZero(s.ProjectPageType)
}

func (s *PinAccessSuite) SetupTest() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.pin WHERE id_issue = ANY($1)", []int64{s.IssueA, s.IssueB})
}

func (s *PinAccessSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.pin WHERE id_issue = ANY($1)", []int64{s.IssueA, s.IssueB})
	for _, p := range []int64{s.ProjectA, s.ProjectB} {
		s.App.Pool.Exec(context.Background(), "DELETE FROM issues.issue WHERE id_project = $1", p)
		s.App.Pool.Exec(context.Background(), "DELETE FROM projects.project WHERE id_project = $1", p)
	}
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email IN ('pinowner@test.sk','pinc@test.sk','pinu@test.sk')")
}

func (s *PinAccessSuite) createIssue(idProject int64, title string) int64 {
	res := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		fmt.Sprintf(`{"title":%q,"description":"desc","estimated":0}`, title), s.OToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var i model.Issue
	json.NewDecoder(res.Body).Decode(&i)
	return i.IdIssue
}

func (s *PinAccessSuite) createPin(token string, idIssue, dest int64, typeID int) int {
	res := Request(s.T(), s.App, "POST", "/api/private/pin",
		fmt.Sprintf(`{"idIssue":%d,"idPinDestination":%d,"idPinDestinationType":%d}`, idIssue, dest, typeID),
		token)
	return res.StatusCode
}

func (s *PinAccessSuite) getPinnedIssueIds(token string, dest int64, typeID int) []int64 {
	res := Request(s.T(), s.App, "GET",
		fmt.Sprintf("/api/private/pin?idPinDestination=%d&idPinDestinationType=%d", dest, typeID),
		"", token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var pins []model.Pin
	s.Require().Nil(json.NewDecoder(res.Body).Decode(&pins))
	ids := make([]int64, 0, len(pins))
	for _, p := range pins {
		ids = append(ids, p.IdIssue)
	}
	return ids
}

// Accepting the pin and hiding it on read would still confirm which issue ids
// exist, via whether the insert passes the foreign key.
func (s *PinAccessSuite) Test_01_UserPage_ForeignIssue_Rejected() {
	s.Equal(http.StatusForbidden, s.createPin(s.UToken, s.IssueA, s.UID, s.UserPageType))
	s.Empty(s.getPinnedIssueIds(s.UToken, s.UID, s.UserPageType))
}

// Read filtering must still hold for pins the create check cannot catch — access
// revoked after pinning. Inserted directly, since the API now refuses it.
func (s *PinAccessSuite) Test_01b_UserPage_ForeignIssue_FilteredOnRead() {
	_, err := s.App.Pool.Exec(context.Background(), `
		INSERT INTO issues.pin (id_issue, id_pin_destination_type, id_pin_destination)
		VALUES ($1, $2, $3)`,
		s.IssueA, s.UserPageType, s.UID)
	s.Require().NoError(err)

	ids := s.getPinnedIssueIds(s.UToken, s.UID, s.UserPageType)
	s.NotContains(ids, s.IssueA)
	s.Empty(ids)
}

// On a shared project-page, a member who cannot read a foreign pinned issue's project must not
// see it, while issues from a project they can read remain visible.
func (s *PinAccessSuite) Test_02_ProjectPage_CrossViewer_Filtered() {
	s.Equal(http.StatusOK, s.createPin(s.OToken, s.IssueA, s.ProjectA, s.ProjectPageType))
	s.Equal(http.StatusOK, s.createPin(s.OToken, s.IssueB, s.ProjectA, s.ProjectPageType))

	// C is a member of A only.
	cIds := s.getPinnedIssueIds(s.CToken, s.ProjectA, s.ProjectPageType)
	s.Contains(cIds, s.IssueA)
	s.NotContains(cIds, s.IssueB)

	// The owner can read both projects, so sees both pins.
	oIds := s.getPinnedIssueIds(s.OToken, s.ProjectA, s.ProjectPageType)
	s.Contains(oIds, s.IssueA)
	s.Contains(oIds, s.IssueB)
}

func Test_RunPinAccessSuite(t *testing.T) {
	suite.Run(t, new(PinAccessSuite))
}
