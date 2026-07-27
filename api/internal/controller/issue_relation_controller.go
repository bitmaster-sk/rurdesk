package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IssueRelationController struct {
	relationRepo *repository.IssueRelationRepository
	issueRepo    *repository.IssueRepository
	projectRepo  *repository.ProjectRepository
	acl          *service.AclService
	notifier     *notify.Notifier
	pool         *pgxpool.Pool
}

func NewIssueRelationController(
	rr *repository.IssueRelationRepository,
	ir *repository.IssueRepository,
	pr *repository.ProjectRepository,
	acl *service.AclService,
	notifier *notify.Notifier,
	pool *pgxpool.Pool,
) *IssueRelationController {
	return &IssueRelationController{relationRepo: rr, issueRepo: ir, projectRepo: pr, acl: acl, notifier: notifier, pool: pool}
}

func (ic *IssueRelationController) GetRelationsBulk(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !ic.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	var idsIssue []int64
	if raw := c.Query("idsIssue"); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil {
				_ = c.Error(err)
				c.Status(http.StatusBadRequest)
				return
			}
			idsIssue = append(idsIssue, id)
		}
	}

	views, err := ic.relationRepo.LoadRelations(ctx, &model.LoadRelationsFilter{
		IdsProject: []int64{idProject},
		IdsIssue:   idsIssue,
	})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, views)
}

func (ic *IssueRelationController) GetRelations(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idIssuePublic, err := strconv.ParseInt(c.Param("idIssuePublic"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !ic.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	issue, err := ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{
		IdProject:     &idProject,
		IdIssuePublic: &idIssuePublic,
	})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}

	views, err := ic.relationRepo.LoadRelations(ctx, &model.LoadRelationsFilter{
		IdsProject: []int64{idProject},
		IdsIssue:   []int64{issue.IdIssue},
	})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, views)
}

func (ic *IssueRelationController) CreateRelation(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idIssuePublic, err := strconv.ParseInt(c.Param("idIssuePublic"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.CreateIssueRelationReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if idIssuePublic == dto.IdIssuePublicTo {
		c.Status(http.StatusBadRequest)
		return
	}
	if dto.RelationType == model.RelationTypeSchedule && dto.RelationSubType == nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if dto.RelationType != model.RelationTypeSchedule {
		if dto.RelationSubType != nil || dto.LagMinutes != nil {
			c.Status(http.StatusBadRequest)
			return
		}
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var result *model.IssueRelation
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		if !ic.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
			return errForbidden
		}

		issueFrom, err := ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{
			IdProject:     &idProject,
			IdIssuePublic: &idIssuePublic,
		})
		if err != nil {
			return err
		}

		issueTo, err := ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{
			IdProject:     &idProject,
			IdIssuePublic: &dto.IdIssuePublicTo,
		})
		if err != nil {
			return err
		}

		from, to := issueFrom.IdIssue, issueTo.IdIssue

		if dto.RelationType == model.RelationTypeDuplicates || dto.RelationType == model.RelationTypeRelatesTo {
			from, to = canonicalize(from, to)
		}

		if dto.RelationType == model.RelationTypeHierarchy || dto.RelationType == model.RelationTypeSchedule {
			cycle, err := ic.relationRepo.HasCycle(ctx, dto.RelationType, from, to)
			if err != nil {
				return err
			}
			if cycle {
				return errCycle
			}
		}

		result, err = ic.relationRepo.InsertRelation(ctx, &model.IssueRelation{
			IdProject:       idProject,
			IdIssueFrom:     from,
			IdIssueTo:       to,
			RelationType:    dto.RelationType,
			RelationSubType: dto.RelationSubType,
			LagMinutes:      dto.LagMinutes,
			CreatedBy:       user.IdUser,
		})
		return err
	})

	if err == errForbidden {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err == errCycle {
		_ = c.Error(errCycle)
		c.Status(http.StatusUnprocessableEntity)
		return
	}
	if err != nil {
		if isConflict(err) {
			_ = c.Error(err)
			c.Status(http.StatusConflict)
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			_ = c.Error(err)
			c.Status(http.StatusNotFound)
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	views, err := ic.relationRepo.LoadRelationById(ctx, result.IdIssueRelation)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// Notify other project members over WebSocket for incremental client updates.
	members, err := ic.projectRepo.LoadProjectsMembers(ctx, []int64{idProject})
	if err == nil {
		for _, member := range members {
			if member.IdUser == user.IdUser {
				continue
			}
			ic.notifier.Send <- &notify.Notice{
				IdUser:  member.IdUser,
				Subject: notify.SubjectRelation,
				Action:  notify.ActionCreate,
				Payload: views,
			}
		}
	}

	c.JSON(http.StatusOK, views)
}

func (ic *IssueRelationController) DeleteRelation(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idRelation, err := strconv.ParseInt(c.Param("idRelation"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		if !ic.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
			return errForbidden
		}
		return ic.relationRepo.DeleteRelation(ctx, idRelation, idProject)
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

func (irc *IssueRelationController) UpdateRelation(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idRelation, err := strconv.ParseInt(c.Param("idRelation"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !irc.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	var dto struct {
		LagMinutes *int64 `json:"lagMinutes"`
	}
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	relation, err := irc.relationRepo.LoadRelation(ctx, idRelation)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if relation == nil {
		c.Status(http.StatusNotFound)
		return
	}
	if relation.IdProject != idProject {
		// Relation belongs to another project — do not leak its existence.
		c.Status(http.StatusNotFound)
		return
	}
	if relation.RelationType != model.RelationTypeSchedule {
		_ = c.Error(fmt.Errorf("only schedule relations can be updated"))
		c.Status(http.StatusBadRequest)
		return
	}

	updated, err := irc.relationRepo.UpdateRelation(ctx, idRelation, dto.LagMinutes)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, updated)
}

// canonicalize ensures non-directional relations always store min(a,b) as from.
func canonicalize(a, b int64) (int64, int64) {
	if a > b {
		return b, a
	}
	return a, b
}

// isConflict detects PostgreSQL unique constraint violations (SQLSTATE 23505).
func isConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
