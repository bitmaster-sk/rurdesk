package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type SeverityNotificationSuite struct {
	suite.Suite
	App    *issue.Application
	Token1 string
	Token2 string
	User1  model.User
	User2  model.User
}

func (s *SeverityNotificationSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token1 = Token(s.T(), s.App)

	// Get User1
	res := Request(s.T(), s.App, "GET", "/api/private/user", "", s.Token1)
	json.NewDecoder(res.Body).Decode(&s.User1)

	// Register User2 (via admin; public registration closed post-bootstrap)
	rb := `{"name":"tester2","email":"test2@test.sk","password":"kreslo"}`
	res = Request(s.T(), s.App, "POST", "/api/private/admin/user", rb, Token(s.T(), s.App))
	s.Require().Contains([]int{http.StatusOK, http.StatusConflict}, res.StatusCode)

	// Login User2
	rb = `{"email":"test2@test.sk","password":"kreslo"}`
	res = Request(s.T(), s.App, "POST", "/api/public/login", rb, "")
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var tk2 struct {
		Token string `json:"token"`
	}
	json.NewDecoder(res.Body).Decode(&tk2)
	s.Token2 = tk2.Token

	// Get User2
	res = Request(s.T(), s.App, "GET", "/api/private/user", "", s.Token2)
	json.NewDecoder(res.Body).Decode(&s.User2)
}

func (s *SeverityNotificationSuite) TestSeverityNotification() {
	// 1. User 1 creates project
	rb := `{"name":"Test Project","color":"#ff0000"}`
	res := Request(s.T(), s.App, "POST", "/api/private/project", rb, s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)
	var project model.Project
	err := json.NewDecoder(res.Body).Decode(&project)
	s.NoError(err)

	// 2. User 1 adds User 2 to project
	rb = fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.User2.IdUser)
	res = Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/member/user", project.IdProject), rb, s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)

	// 3. Get severities
	res = Request(s.T(), s.App, "GET", "/api/private/severity", "", s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)
	var severities []*model.Severity
	err = json.NewDecoder(res.Body).Decode(&severities)
	s.NoError(err)

	var low, high *model.Severity
	for _, sev := range severities {
		if sev.IdProject == project.IdProject {
			if sev.Title == "Low" {
				low = sev
			} else if sev.Title == "High" {
				high = sev
			}
		}
	}
	s.NotNil(low)
	s.NotNil(high)

	// 4. User 1 creates issue
	rb = fmt.Sprintf(`{"title":"Test Issue","description":"desc","idSeverity":%d}`, high.IdSeverity)
	res = Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/issue", project.IdProject), rb, s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	err = json.NewDecoder(res.Body).Decode(&iss)
	s.NoError(err)

	// 5. User 2 downgrades severity to Low
	rb = fmt.Sprintf(`{"title":"Test Issue","description":"desc","idSeverity":%d}`, low.IdSeverity)
	res = Request(s.T(), s.App, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/%d", project.IdProject, iss.IdIssuePublic), rb, s.Token2)
	s.Equal(http.StatusOK, res.StatusCode)

	// 6. User 1 checks notifications
	res = Request(s.T(), s.App, "GET", "/api/private/notification", "", s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)
	var notifications []*model.Notification
	err = json.NewDecoder(res.Body).Decode(&notifications)
	s.NoError(err)

	foundDeescalated := false
	for _, n := range notifications {
		if n.RefId == fmt.Sprintf("%d", iss.IdIssue) {
			s.T().Logf("Notification type: %s", n.Type)
			if n.Type == "severity_deescalated" {
				foundDeescalated = true
			}
		}
	}
	s.True(foundDeescalated, "Should have found a de-escalated notification for downgrade")
}

func (s *SeverityNotificationSuite) TestSeverityEscalationNotification() {
	// 1. User 1 creates project
	rb := `{"name":"Test Project 2","color":"#00ff00"}`
	res := Request(s.T(), s.App, "POST", "/api/private/project", rb, s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)
	var project model.Project
	err := json.NewDecoder(res.Body).Decode(&project)
	s.NoError(err)

	// 2. User 1 adds User 2 to project
	rb = fmt.Sprintf(`{"idUser":%d,"role":"member"}`, s.User2.IdUser)
	res = Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/member/user", project.IdProject), rb, s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)

	// 3. Get severities
	res = Request(s.T(), s.App, "GET", "/api/private/severity", "", s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)
	var severities []*model.Severity
	err = json.NewDecoder(res.Body).Decode(&severities)
	s.NoError(err)

	var low, high *model.Severity
	for _, sev := range severities {
		if sev.IdProject == project.IdProject {
			if sev.Title == "Low" {
				low = sev
			} else if sev.Title == "High" {
				high = sev
			}
		}
	}
	s.NotNil(low)
	s.NotNil(high)

	// 4. User 1 creates issue with Low severity
	rb = fmt.Sprintf(`{"title":"Test Issue","description":"desc","idSeverity":%d}`, low.IdSeverity)
	res = Request(s.T(), s.App, "POST", fmt.Sprintf("/api/private/project/%d/issue", project.IdProject), rb, s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)
	var iss model.Issue
	err = json.NewDecoder(res.Body).Decode(&iss)
	s.NoError(err)

	// 5. User 2 upgrades severity to High
	rb = fmt.Sprintf(`{"title":"Test Issue","description":"desc","idSeverity":%d}`, high.IdSeverity)
	res = Request(s.T(), s.App, "PATCH", fmt.Sprintf("/api/private/project/%d/issue/%d", project.IdProject, iss.IdIssuePublic), rb, s.Token2)
	s.Equal(http.StatusOK, res.StatusCode)

	// 6. User 1 checks notifications
	res = Request(s.T(), s.App, "GET", "/api/private/notification", "", s.Token1)
	s.Equal(http.StatusOK, res.StatusCode)
	var notifications []*model.Notification
	err = json.NewDecoder(res.Body).Decode(&notifications)
	s.NoError(err)

	foundEscalated := false
	for _, n := range notifications {
		if n.RefId == fmt.Sprintf("%d", iss.IdIssue) {
			s.T().Logf("Notification type: %s", n.Type)
			if n.Type == "severity_escalated" {
				foundEscalated = true
			}
		}
	}
	s.True(foundEscalated, "Should have found an escalated notification for upgrade")
}

func TestSeverityNotificationSuite(t *testing.T) {
	suite.Run(t, new(SeverityNotificationSuite))
}
