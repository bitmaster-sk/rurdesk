package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxTrackedSeconds int64 = 24 * 3600

func validateTrackTimes(tracked *int64, startAt *time.Time, endAt *time.Time) error {
	if tracked != nil {
		if *tracked < 0 {
			return errors.New("tracked duration must not be negative")
		}
		if *tracked > maxTrackedSeconds {
			return errors.New("tracked duration exceeds the maximum of 86400 seconds")
		}
		return nil
	}
	if startAt == nil {
		return errors.New("startAt is required when tracked is not provided")
	}
	effectiveEndAt := time.Now().UTC()
	if endAt != nil {
		effectiveEndAt = *endAt
	}
	if effectiveEndAt.Before(*startAt) {
		return errors.New("endAt must not be before startAt")
	}
	trackedDuration := int64(effectiveEndAt.Sub(*startAt).Seconds())
	if trackedDuration > maxTrackedSeconds {
		return errors.New("tracked duration exceeds the maximum of 86400 seconds")
	}
	return nil
}

type TrackerController struct {
	acl         *service.AclService
	trackerRepo *repository.TrackerRepository
	projectRepo *repository.ProjectRepository
	issueRepo   *repository.IssueRepository
	pool        *pgxpool.Pool
}

func NewTrackerController(
	acl *service.AclService,
	tr *repository.TrackerRepository,
	pr *repository.ProjectRepository,
	ir *repository.IssueRepository,
	pool *pgxpool.Pool,
) *TrackerController {
	return &TrackerController{
		acl:         acl,
		trackerRepo: tr,
		projectRepo: pr,
		issueRepo:   ir,
		pool:        pool,
	}
}

func (tc *TrackerController) GetTracker(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	tracker, err := tc.trackerRepo.LoadTracker(ctx, user.IdUser)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusOK, gin.H{})
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, tracker)
}

func (tc *TrackerController) CreateTracker(c *gin.Context) {
	var dto model.CreateTrackerReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	issue, err := tc.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &dto.IdProject, IdIssuePublic: &dto.IdIssuePublic})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	project, err := tc.projectRepo.LoadProjectByIssue(ctx, issue.IdIssue)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	if !tc.acl.CanUpdateIssue(ctx, user.IdUser, project.IdProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	_, err = tc.trackerRepo.LoadTracker(ctx, user.IdUser)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if err == nil {
		_ = c.Error(errs.ErrConflict)
		c.Status(http.StatusConflict)
		return
	}

	tracker := &model.Tracker{
		IdUser:        user.IdUser,
		IdIssue:       issue.IdIssue,
		IdIssuePublic: issue.IdIssuePublic,
		IdProject:     issue.IdProject,
	}

	if _, err = tc.trackerRepo.InsertTracker(ctx, tracker); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// Reload so the response carries the joined issue title and project name.
	tracker, err = tc.trackerRepo.LoadTracker(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, tracker)
}

func (tc *TrackerController) DeleteTracker(c *gin.Context) {
	idTracker, err := strconv.ParseInt(c.Param("idTracker"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	tracker, err := tc.trackerRepo.LoadTracker(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if tracker.IdTracker != idTracker {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}

	if err = tc.trackerRepo.DeleteTracker(ctx, idTracker); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (tc *TrackerController) PauseTracker(c *gin.Context) {
	tc.switchPause(c, true)
}

func (tc *TrackerController) ResumeTracker(c *gin.Context) {
	tc.switchPause(c, false)
}

func (tc *TrackerController) switchPause(c *gin.Context, pause bool) {
	idTracker, err := strconv.ParseInt(c.Param("idTracker"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var tracker *model.Tracker
	err = extctx.RunInTx(ctx, tc.pool, func(ctx context.Context) error {
		tracker, err = tc.trackerRepo.LoadTracker(ctx, user.IdUser)
		if err != nil {
			return err
		}
		if tracker.IdTracker != idTracker {
			return errs.ErrNotFound
		}
		now := time.Now().UTC()
		if pause {
			if err := tc.trackerRepo.PauseTracker(ctx, idTracker, now); err != nil {
				return err
			}
		} else if err := tc.trackerRepo.ResumeTracker(ctx, idTracker, now); err != nil {
			return err
		}
		tracker, err = tc.trackerRepo.LoadTracker(ctx, user.IdUser)
		return err
	})
	if errors.Is(err, errs.ErrNotFound) {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, tracker)
}

func (tc *TrackerController) SubmitTracker(c *gin.Context) {
	idTracker, err := strconv.ParseInt(c.Param("idTracker"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.SubmitTrackerReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&dto); err != nil {
			_ = c.Error(err)
			c.Status(http.StatusBadRequest)
			return
		}
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var track *model.Track
	err = extctx.RunInTx(ctx, tc.pool, func(ctx context.Context) error {
		tracker, err := tc.trackerRepo.LoadTracker(ctx, user.IdUser)
		if err != nil {
			return err
		}
		if tracker.IdTracker != idTracker {
			return errs.ErrForbidden
		}

		track = tracker.ToTrack()
		now := time.Now().UTC()
		track.Note = dto.Note

		// A forgotten timer must not write an unbounded value into issue.tracked, so the
		// entry is clamped to the same ceiling the manual endpoints enforce.
		tracked := tracker.ElapsedSeconds(now)
		if tracked > maxTrackedSeconds {
			tracked = maxTrackedSeconds
		}
		endAt := track.StartAt.Add(time.Duration(tracked) * time.Second)
		track.EndAt = &endAt
		track.Tracked = &tracked

		track, err = tc.trackerRepo.InsertTrack(ctx, track)
		if err != nil {
			return err
		}
		if err := tc.trackerRepo.UpdateIssueTracked(ctx, tracker.IdIssue); err != nil {
			return err
		}
		return tc.trackerRepo.DeleteTracker(ctx, tracker.IdTracker)
	})
	if err == errs.ErrForbidden {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, track)
}

func (tc *TrackerController) GetTracks(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	filter := model.TracksFilter{}

	if v := c.Query("idProject"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.IdsProject = []int64{n}
		}
	}
	if v := c.Query("idIssue"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.IdIssue = &n
		}
	}
	if v := c.Query("idUser"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			filter.IdUser = &n
		}
	}

	if len(filter.IdsProject) == 1 {
		if !tc.acl.CanReadProject(ctx, user.IdUser, filter.IdsProject[0]) {
			_ = c.Error(errs.ErrForbidden)
			c.Status(http.StatusForbidden)
			return
		}
	} else {
		idsProject, err := tc.acl.LoadVisibleProjectIds(ctx, user.IdUser)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		filter.IdsProject = idsProject
	}

	if filter.IdIssue != nil {
		issueProject, err := tc.projectRepo.LoadProjectByIssue(ctx, *filter.IdIssue)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if !tc.acl.CanReadProject(ctx, user.IdUser, issueProject.IdProject) {
			_ = c.Error(errs.ErrForbidden)
			c.Status(http.StatusForbidden)
			return
		}
	}

	tracks, err := tc.trackerRepo.LoadTracks(ctx, filter)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, tracks)
}

func (tc *TrackerController) CreateTrack(c *gin.Context) {
	var dto model.CreateTrackReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	if err := validateTrackTimes(dto.Tracked, dto.StartAt, dto.EndAt); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var track *model.Track
	err := extctx.RunInTx(ctx, tc.pool, func(ctx context.Context) error {
		issueProject, projectErr := tc.projectRepo.LoadProjectByIssue(ctx, dto.IdIssue)
		if projectErr != nil {
			return projectErr
		}
		if !tc.acl.CanUpdateIssue(ctx, user.IdUser, issueProject.IdProject) {
			return errs.ErrForbidden
		}

		t := &model.Track{
			IdUser:  user.IdUser,
			IdIssue: dto.IdIssue,
			Tracked: dto.Tracked,
			StartAt: dto.StartAt,
			EndAt:   dto.EndAt,
			Note:    dto.Note,
		}

		if t.Tracked != nil {
			endAt := time.Now().UTC()
			if t.EndAt != nil {
				endAt = *t.EndAt
			}
			startAt := endAt.Add(-time.Duration(*t.Tracked) * time.Second)
			t.EndAt = &endAt
			t.StartAt = &startAt
		} else {
			if t.EndAt == nil {
				endAt := time.Now().UTC()
				t.EndAt = &endAt
			}
			tracked := int64(t.EndAt.Sub(*t.StartAt).Seconds())
			t.Tracked = &tracked
		}

		var err error
		track, err = tc.trackerRepo.InsertTrack(ctx, t)
		if err != nil {
			return err
		}
		return tc.trackerRepo.UpdateIssueTracked(ctx, track.IdIssue)
	})
	if err == errs.ErrForbidden {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, track)
}

func (tc *TrackerController) EditTrack(c *gin.Context) {
	idTrack, err := strconv.ParseInt(c.Param("idTrack"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.EditTrackReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	dto.IdTrack = idTrack

	if err := validateTrackTimes(dto.Tracked, dto.StartAt, dto.EndAt); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var track *model.Track
	err = extctx.RunInTx(ctx, tc.pool, func(ctx context.Context) error {
		t, err := tc.trackerRepo.LoadTrack(ctx, dto.IdTrack)
		if err != nil {
			return err
		}
		issueProject, err := tc.projectRepo.LoadProjectByIssue(ctx, t.IdIssue)
		if err != nil {
			return err
		}
		if !tc.acl.CanUpdateIssue(ctx, user.IdUser, issueProject.IdProject) {
			return errs.ErrForbidden
		}
		if !tc.canMutateTrack(ctx, user.IdUser, t, issueProject.IdProject) {
			return errs.ErrForbidden
		}

		t.Tracked = dto.Tracked
		t.StartAt = dto.StartAt
		t.EndAt = dto.EndAt
		t.Note = dto.Note

		if t.Tracked != nil {
			endAt := time.Now().UTC()
			if t.EndAt != nil {
				endAt = *t.EndAt
			}
			startAt := endAt.Add(-time.Duration(*t.Tracked) * time.Second)
			t.EndAt = &endAt
			t.StartAt = &startAt
		} else {
			if t.EndAt == nil {
				endAt := time.Now().UTC()
				t.EndAt = &endAt
			}
			tracked := int64(t.EndAt.Sub(*t.StartAt).Seconds())
			t.Tracked = &tracked
		}

		track, err = tc.trackerRepo.UpdateTrack(ctx, t)
		if err != nil {
			return err
		}
		return tc.trackerRepo.UpdateIssueTracked(ctx, track.IdIssue)
	})
	if err == errs.ErrForbidden {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, track)
}

// canMutateTrack reports whether the caller authored the entry or is a project owner
// (who may correct or clean up any member's entry). Callers must already have
// verified project update access (CanUpdateIssue).
func (tc *TrackerController) canMutateTrack(ctx context.Context, idUser int64, track *model.Track, idProject int64) bool {
	if track.IdUser == idUser {
		return true
	}
	return tc.acl.CanUpdateProject(ctx, idUser, idProject)
}

func (tc *TrackerController) DeleteTrack(c *gin.Context) {
	idTrack, err := strconv.ParseInt(c.Param("idTrack"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	err = extctx.RunInTx(ctx, tc.pool, func(ctx context.Context) error {
		track, err := tc.trackerRepo.LoadTrack(ctx, idTrack)
		if err != nil {
			return err
		}
		project, err := tc.projectRepo.LoadProjectByIssue(ctx, track.IdIssue)
		if err != nil {
			return err
		}
		if !tc.acl.CanUpdateIssue(ctx, user.IdUser, project.IdProject) {
			return errs.ErrForbidden
		}
		if !tc.canMutateTrack(ctx, user.IdUser, track, project.IdProject) {
			return errs.ErrForbidden
		}
		if err := tc.trackerRepo.DeleteTrack(ctx, idTrack); err != nil {
			return err
		}
		return tc.trackerRepo.UpdateIssueTracked(ctx, track.IdIssue)
	})
	if err == errs.ErrForbidden {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}
