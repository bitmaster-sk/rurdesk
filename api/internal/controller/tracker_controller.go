package controller

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

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
		_ = c.Error(errForbidden)
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
		c.Status(http.StatusConflict)
		return
	}

	tracker := &model.Tracker{
		IdUser:        user.IdUser,
		IdIssue:       issue.IdIssue,
		IdIssuePublic: issue.IdIssuePublic,
		IdProject:     issue.IdProject,
	}

	tracker, err = tc.trackerRepo.InsertTracker(ctx, tracker)
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

func (tc *TrackerController) SubmitTracker(c *gin.Context) {
	idTracker, err := strconv.ParseInt(c.Param("idTracker"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
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
			return errForbidden
		}

		track = tracker.ToTrack()
		endAt := time.Now().UTC()
		track.EndAt = &endAt
		tracked := int64(track.EndAt.Sub(*track.StartAt).Seconds())
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
	if err == errForbidden {
		_ = c.Error(errForbidden)
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
			_ = c.Error(errForbidden)
			c.Status(http.StatusForbidden)
			return
		}
	} else {
		idsProject, err := tc.projectRepo.LoadProjectsIds(ctx, user.IdUser)
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
			_ = c.Error(errForbidden)
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
			return errForbidden
		}

		t := &model.Track{
			IdUser:  user.IdUser,
			IdIssue: dto.IdIssue,
			Tracked: dto.Tracked,
			StartAt: dto.StartAt,
			EndAt:   dto.EndAt,
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
	if err == errForbidden {
		_ = c.Error(errForbidden)
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
			return errForbidden
		}
		if !tc.canMutateTrack(ctx, user.IdUser, t, issueProject.IdProject) {
			return errForbidden
		}

		t.Tracked = dto.Tracked
		t.StartAt = dto.StartAt
		t.EndAt = dto.EndAt

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
	if err == errForbidden {
		_ = c.Error(errForbidden)
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
			return errForbidden
		}
		if !tc.canMutateTrack(ctx, user.IdUser, track, project.IdProject) {
			return errForbidden
		}
		if err := tc.trackerRepo.DeleteTrack(ctx, idTrack); err != nil {
			return err
		}
		return tc.trackerRepo.UpdateIssueTracked(ctx, track.IdIssue)
	})
	if err == errForbidden {
		_ = c.Error(errForbidden)
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
