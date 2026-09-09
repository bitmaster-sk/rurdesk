package model

import "time"

type Tracker struct {
	IdTracker     int64      `json:"idTracker" db:"id_tracker"`
	IdUser        int64      `json:"idUser" db:"id_user"`
	IdIssue       int64      `json:"idIssue" db:"id_issue"`
	StartAt       time.Time  `json:"startAt" db:"start_at"`
	PausedAt      *time.Time `json:"pausedAt" db:"paused_at"`
	PausedSeconds int64      `json:"pausedSeconds" db:"paused_seconds"`
	IdProject     int64      `json:"idProject" db:"id_project"`
	IdIssuePublic int64      `json:"idIssuePublic" db:"id_issue_public"`
	IssueTitle    string     `json:"issueTitle" db:"issue_title"`
	ProjectName   string     `json:"projectName" db:"project_name"`
}

// ElapsedSeconds is the tracked time with paused stretches removed. It never goes
// negative even if the clock is skewed or the row is edited by hand.
func (t *Tracker) ElapsedSeconds(now time.Time) int64 {
	until := now
	if t.PausedAt != nil {
		until = *t.PausedAt
	}
	elapsed := int64(until.Sub(t.StartAt).Seconds()) - t.PausedSeconds
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

type Track struct {
	IdTrack       int64      `json:"idTrack" db:"id_track"`
	IdUser        int64      `json:"idUser" db:"id_user"`
	IdIssue       int64      `json:"idIssue" db:"id_issue"`
	IdIssuePublic int64      `json:"idIssuePublic" db:"id_issue_public"`
	IdProject     int64      `json:"idProject" db:"id_project"`
	IssueTitle    string     `json:"issueTitle" db:"issue_title"`
	Tracked       *int64     `json:"tracked" db:"tracked"`
	StartAt       *time.Time `json:"startAt" db:"start_at"`
	EndAt         *time.Time `json:"endAt" db:"end_at"`
	Note          *string    `json:"note" db:"note"`
}

func (t *Tracker) ToTrack() *Track {
	return &Track{
		IdUser:  t.IdUser,
		IdIssue: t.IdIssue,
		StartAt: &t.StartAt,
	}
}

type CreateTrackerReq struct {
	IdIssuePublic int64 `json:"idIssuePublic" binding:"required"`
	IdProject     int64 `json:"idProject" binding:"required"`
}

type SubmitTrackerReq struct {
	Note *string `json:"note" binding:"omitempty,max=2000"`
}

type CreateTrackReq struct {
	IdIssue int64      `json:"idIssue"  binding:"required"`
	Tracked *int64     `json:"tracked"  binding:"omitempty"`
	StartAt *time.Time `json:"startAt"`
	EndAt   *time.Time `json:"endAt"`
	Note    *string    `json:"note" binding:"omitempty,max=2000"`
}

type EditTrackReq struct {
	IdTrack int64      `json:"idTrack"`
	Tracked *int64     `json:"tracked"  binding:"omitempty"`
	StartAt *time.Time `json:"startAt"`
	EndAt   *time.Time `json:"endAt"`
	Note    *string    `json:"note" binding:"omitempty,max=2000"`
}

type TracksFilter struct {
	IdsProject []int64
	IdIssue    *int64
	IdUser     *int64
	StartFrom  *time.Time
	StartTo    *time.Time
}

type GetTracksReq struct {
	IdProject *int64     `json:"idProject"`
	IdIssue   *int64     `json:"idIssue"`
	IdUser    *int64     `json:"idUser"`
	StartFrom *time.Time `json:"startFrom"`
	StartTo   *time.Time `json:"startTo"`
}

func (gtd *GetTracksReq) ToTracksFilter() TracksFilter {
	idsProject := []int64{}
	if gtd.IdProject != nil {
		idsProject = append(idsProject, *gtd.IdProject)
	}
	return TracksFilter{
		IdsProject: idsProject,
		IdIssue:    gtd.IdIssue,
		IdUser:     gtd.IdUser,
		StartFrom:  gtd.StartFrom,
		StartTo:    gtd.StartTo,
	}
}
