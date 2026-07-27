package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

type TrackerValidationSuite struct {
	suite.Suite
	App       *issue.Application
	Token     string
	IdIssue   int64
	IdProject int64
}

func (s *TrackerValidationSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	res := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"tracker-validation-test-project"}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var proj struct {
		IdProject int64 `json:"idProject"`
	}
	json.NewDecoder(res.Body).Decode(&proj)
	s.IdProject = proj.IdProject

	issRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"Tracker test issue","description":"Issue for tracker validation tests."}`,
		s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	json.NewDecoder(issRes.Body).Decode(&iss)
	s.IdIssue = iss.IdIssue
}

func (s *TrackerValidationSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.track WHERE id_issue = $1", s.IdIssue)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM issues.issue WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
}

func (s *TrackerValidationSuite) trackURL() string {
	return "/api/private/track"
}

func (s *TrackerValidationSuite) editTrackURL(idTrack int64) string {
	return fmt.Sprintf("/api/private/track/%d", idTrack)
}

func (s *TrackerValidationSuite) createValidTrack() int64 {
	tracked := int64(1800)
	body := fmt.Sprintf(`{"idIssue":%d,"tracked":%d}`, s.IdIssue, tracked)
	res := Request(s.T(), s.App, "POST", s.trackURL(), body, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var track model.Track
	json.NewDecoder(res.Body).Decode(&track)
	return track.IdTrack
}

// --- CreateTrack validation tests ---

func (s *TrackerValidationSuite) Test_CreateTrack_EndBeforeStart_Returns400() {
	start := time.Now().UTC().Add(-time.Hour)
	end := start.Add(-30 * time.Minute)
	body := fmt.Sprintf(`{"idIssue":%d,"startAt":"%s","endAt":"%s"}`,
		s.IdIssue, start.Format(time.RFC3339), end.Format(time.RFC3339))
	res := Request(s.T(), s.App, "POST", s.trackURL(), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_CreateTrack_NegativeTracked_Returns400() {
	body := fmt.Sprintf(`{"idIssue":%d,"tracked":-1}`, s.IdIssue)
	res := Request(s.T(), s.App, "POST", s.trackURL(), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_CreateTrack_TrackedExceedsMax_Returns400() {
	body := fmt.Sprintf(`{"idIssue":%d,"tracked":86401}`, s.IdIssue)
	res := Request(s.T(), s.App, "POST", s.trackURL(), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_CreateTrack_SpanExceedsMax_Returns400() {
	start := time.Now().UTC().Add(-25 * time.Hour)
	end := time.Now().UTC()
	body := fmt.Sprintf(`{"idIssue":%d,"startAt":"%s","endAt":"%s"}`,
		s.IdIssue, start.Format(time.RFC3339), end.Format(time.RFC3339))
	res := Request(s.T(), s.App, "POST", s.trackURL(), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_CreateTrack_NoStartNoTracked_Returns400() {
	body := fmt.Sprintf(`{"idIssue":%d}`, s.IdIssue)
	res := Request(s.T(), s.App, "POST", s.trackURL(), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_CreateTrack_ValidTracked_Returns200() {
	body := fmt.Sprintf(`{"idIssue":%d,"tracked":3600}`, s.IdIssue)
	res := Request(s.T(), s.App, "POST", s.trackURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_CreateTrack_ValidStartEnd_Returns200() {
	start := time.Now().UTC().Add(-2 * time.Hour)
	end := time.Now().UTC().Add(-time.Hour)
	body := fmt.Sprintf(`{"idIssue":%d,"startAt":"%s","endAt":"%s"}`,
		s.IdIssue, start.Format(time.RFC3339), end.Format(time.RFC3339))
	res := Request(s.T(), s.App, "POST", s.trackURL(), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)
}

// --- EditTrack validation tests ---

func (s *TrackerValidationSuite) Test_EditTrack_EndBeforeStart_Returns400() {
	idTrack := s.createValidTrack()
	start := time.Now().UTC().Add(-time.Hour)
	end := start.Add(-30 * time.Minute)
	body := fmt.Sprintf(`{"startAt":"%s","endAt":"%s"}`,
		start.Format(time.RFC3339), end.Format(time.RFC3339))
	res := Request(s.T(), s.App, "PATCH", s.editTrackURL(idTrack), body, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_EditTrack_NegativeTracked_Returns400() {
	idTrack := s.createValidTrack()
	res := Request(s.T(), s.App, "PATCH", s.editTrackURL(idTrack), `{"tracked":-1}`, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_EditTrack_TrackedExceedsMax_Returns400() {
	idTrack := s.createValidTrack()
	res := Request(s.T(), s.App, "PATCH", s.editTrackURL(idTrack), `{"tracked":86401}`, s.Token)
	s.Equal(http.StatusBadRequest, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_EditTrack_ValidTracked_Returns200() {
	idTrack := s.createValidTrack()
	res := Request(s.T(), s.App, "PATCH", s.editTrackURL(idTrack), `{"tracked":7200}`, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)
}

func (s *TrackerValidationSuite) Test_EditTrack_ValidStartEnd_Returns200() {
	idTrack := s.createValidTrack()
	start := time.Now().UTC().Add(-3 * time.Hour)
	end := time.Now().UTC().Add(-2 * time.Hour)
	body := fmt.Sprintf(`{"startAt":"%s","endAt":"%s"}`,
		start.Format(time.RFC3339), end.Format(time.RFC3339))
	res := Request(s.T(), s.App, "PATCH", s.editTrackURL(idTrack), body, s.Token)
	s.Equal(http.StatusOK, res.StatusCode)
}

func Test_RunTrackerValidationSuite(t *testing.T) {
	suite.Run(t, new(TrackerValidationSuite))
}
