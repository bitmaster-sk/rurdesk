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

type TrackerPauseSuite struct {
	suite.Suite
	App           *issue.Application
	Token         string
	IdIssue       int64
	IdIssuePublic int64
	IdProject     int64
}

func (s *TrackerPauseSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.Token = Token(s.T(), s.App)

	res := Request(s.T(), s.App, "POST", "/api/private/project",
		`{"name":"tracker-pause-test-project"}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var proj struct {
		IdProject int64 `json:"idProject"`
	}
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&proj))
	s.IdProject = proj.IdProject

	issRes := Request(s.T(), s.App, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", s.IdProject),
		`{"title":"Tracker pause issue","description":"Issue for tracker pause tests."}`,
		s.Token)
	s.Require().Equal(http.StatusOK, issRes.StatusCode)
	var iss model.Issue
	s.Require().NoError(json.NewDecoder(issRes.Body).Decode(&iss))
	s.IdIssue = iss.IdIssue
	s.IdIssuePublic = iss.IdIssuePublic
}

func (s *TrackerPauseSuite) TearDownSuite() {
	ctx := context.Background()
	s.App.Pool.Exec(ctx, "DELETE FROM issues.tracker WHERE id_issue = $1", s.IdIssue)
	s.App.Pool.Exec(ctx, "DELETE FROM issues.track WHERE id_issue = $1", s.IdIssue)
	s.App.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_project = $1", s.IdProject)
	s.App.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", s.IdProject)
}

func (s *TrackerPauseSuite) SetupTest() {
	s.App.Pool.Exec(context.Background(), "DELETE FROM issues.tracker WHERE id_issue = $1", s.IdIssue)
	s.App.Pool.Exec(context.Background(), "DELETE FROM issues.track WHERE id_issue = $1", s.IdIssue)
}

func (s *TrackerPauseSuite) startTracker() model.Tracker {
	body := fmt.Sprintf(`{"idProject":%d,"idIssuePublic":%d}`, s.IdProject, s.IdIssuePublic)
	res := Request(s.T(), s.App, "POST", "/api/private/tracker", body, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)
	var tracker model.Tracker
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&tracker))
	return tracker
}

func (s *TrackerPauseSuite) backdateStart(idTracker int64, ago time.Duration) {
	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE issues.tracker SET start_at = $2 WHERE id_tracker = $1",
		idTracker, time.Now().UTC().Add(-ago))
	s.Require().NoError(err)
}

func (s *TrackerPauseSuite) Test_PauseTracker_MarksPausedAt() {
	tracker := s.startTracker()

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/tracker/%d/pause", tracker.IdTracker), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var paused model.Tracker
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&paused))
	s.Require().NotNil(paused.PausedAt)
	s.Equal(int64(0), paused.PausedSeconds)
}

func (s *TrackerPauseSuite) Test_ResumeTracker_AccumulatesPausedSeconds() {
	tracker := s.startTracker()

	pauseRes := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/tracker/%d/pause", tracker.IdTracker), "", s.Token)
	s.Require().Equal(http.StatusOK, pauseRes.StatusCode)

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE issues.tracker SET paused_at = $2 WHERE id_tracker = $1",
		tracker.IdTracker, time.Now().UTC().Add(-10*time.Minute))
	s.Require().NoError(err)

	resumeRes := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/tracker/%d/resume", tracker.IdTracker), "", s.Token)
	s.Require().Equal(http.StatusOK, resumeRes.StatusCode)

	var resumed model.Tracker
	s.Require().NoError(json.NewDecoder(resumeRes.Body).Decode(&resumed))
	s.Nil(resumed.PausedAt)
	s.GreaterOrEqual(resumed.PausedSeconds, int64(590))
}

func (s *TrackerPauseSuite) Test_SubmitTracker_SubtractsPausedTime() {
	tracker := s.startTracker()
	s.backdateStart(tracker.IdTracker, 2*time.Hour)

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE issues.tracker SET paused_seconds = 3600 WHERE id_tracker = $1", tracker.IdTracker)
	s.Require().NoError(err)

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/tracker/%d/submit", tracker.IdTracker), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var track model.Track
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&track))
	s.Require().NotNil(track.Tracked)
	s.InDelta(3600, *track.Tracked, 10)
}

func (s *TrackerPauseSuite) Test_SubmitTracker_WhilePaused_StopsAtPauseTime() {
	tracker := s.startTracker()
	s.backdateStart(tracker.IdTracker, 3*time.Hour)

	_, err := s.App.Pool.Exec(context.Background(),
		"UPDATE issues.tracker SET paused_at = $2 WHERE id_tracker = $1",
		tracker.IdTracker, time.Now().UTC().Add(-time.Hour))
	s.Require().NoError(err)

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/tracker/%d/submit", tracker.IdTracker), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var track model.Track
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&track))
	s.Require().NotNil(track.Tracked)
	s.InDelta(7200, *track.Tracked, 10)
}

func (s *TrackerPauseSuite) Test_SubmitTracker_ForgottenTimer_ClampedToOneDay() {
	tracker := s.startTracker()
	s.backdateStart(tracker.IdTracker, 90*time.Hour)

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/tracker/%d/submit", tracker.IdTracker), "", s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var track model.Track
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&track))
	s.Require().NotNil(track.Tracked)
	s.Equal(int64(24*3600), *track.Tracked)
	s.Require().NotNil(track.EndAt)
	s.Require().NotNil(track.StartAt)
	s.Equal(int64(24*3600), int64(track.EndAt.Sub(*track.StartAt).Seconds()))
}

func (s *TrackerPauseSuite) Test_SubmitTracker_WithNote_StoresNote() {
	tracker := s.startTracker()

	res := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/tracker/%d/submit", tracker.IdTracker),
		`{"note":"Rebuilt the header control"}`, s.Token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var track model.Track
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&track))
	s.Require().NotNil(track.Note)
	s.Equal("Rebuilt the header control", *track.Note)
}

func (s *TrackerPauseSuite) Test_EditTrack_UpdatesNote() {
	body := fmt.Sprintf(`{"idIssue":%d,"tracked":1800,"note":"first"}`, s.IdIssue)
	createRes := Request(s.T(), s.App, "POST", "/api/private/track", body, s.Token)
	s.Require().Equal(http.StatusOK, createRes.StatusCode)
	var created model.Track
	s.Require().NoError(json.NewDecoder(createRes.Body).Decode(&created))
	s.Require().NotNil(created.Note)
	s.Equal("first", *created.Note)

	editRes := Request(s.T(), s.App, "PATCH",
		fmt.Sprintf("/api/private/track/%d", created.IdTrack),
		`{"tracked":1800,"note":"second"}`, s.Token)
	s.Require().Equal(http.StatusOK, editRes.StatusCode)
	var edited model.Track
	s.Require().NoError(json.NewDecoder(editRes.Body).Decode(&edited))
	s.Require().NotNil(edited.Note)
	s.Equal("second", *edited.Note)
}

func (s *TrackerPauseSuite) Test_PauseTracker_ForeignTracker_Returns404() {
	s.startTracker()

	res := Request(s.T(), s.App, "PATCH", "/api/private/tracker/999999/pause", "", s.Token)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func Test_RunTrackerPauseSuite(t *testing.T) {
	suite.Run(t, new(TrackerPauseSuite))
}
