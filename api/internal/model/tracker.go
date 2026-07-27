package model

import "time"

type Tracker struct {
	IdTracker     int64     `json:"idTracker" db:"id_tracker"`
	IdUser        int64     `json:"idUser" db:"id_user"`
	IdIssue       int64     `json:"idIssue" db:"id_issue"`
	StartAt       time.Time `json:"startAt" db:"start_at"`
	IdProject     int64     `json:"idProject" db:"id_project"`
	IdIssuePublic int64     `json:"idIssuePublic" db:"id_issue_public"`
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

type CreateTrackReq struct {
	IdIssue int64      `json:"idIssue"  binding:"required"`
	Tracked *int64     `json:"tracked"  binding:"omitempty"`
	StartAt *time.Time `json:"startAt"`
	EndAt   *time.Time `json:"endAt"`
}

type EditTrackReq struct {
	IdTrack int64      `json:"idTrack"`
	Tracked *int64     `json:"tracked"  binding:"omitempty"`
	StartAt *time.Time `json:"startAt"`
	EndAt   *time.Time `json:"endAt"`
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
