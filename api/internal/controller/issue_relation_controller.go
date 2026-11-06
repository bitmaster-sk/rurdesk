package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/bitmaster-sk/rurdesk/api/internal/urlutil"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type IssueRelationController struct {
	relationRepo  *repository.IssueRelationRepository
	issueRepo     *repository.IssueRepository
	projectRepo   *repository.ProjectRepository
	acl           *service.AclService
	notifier      notify.INotifier
	hierarchy     *service.HierarchyRollupService
	hierarchyRepo *repository.IssueHierarchyRepository
	valueRepo     *repository.CustomFieldValueRepository
	pool          *pgxpool.Pool
}

func NewIssueRelationController(
	rr *repository.IssueRelationRepository,
	ir *repository.IssueRepository,
	pr *repository.ProjectRepository,
	acl *service.AclService,
	notifier notify.INotifier,
	hierarchy *service.HierarchyRollupService,
	hierarchyRepo *repository.IssueHierarchyRepository,
	valueRepo *repository.CustomFieldValueRepository,
	pool *pgxpool.Pool,
) *IssueRelationController {
	return &IssueRelationController{
		relationRepo:  rr,
		issueRepo:     ir,
		projectRepo:   pr,
		acl:           acl,
		notifier:      notifier,
		hierarchy:     hierarchy,
		hierarchyRepo: hierarchyRepo,
		valueRepo:     valueRepo,
		pool:          pool,
	}
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
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// Shared helper: one array-param code path in the API. Was a local comma-only
	// split returning 400 on a bad element; now skips it, like the issue filters.
	idsIssue := urlutil.ParseInt64Array(c, "idsIssue")

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
		_ = c.Error(errs.ErrForbidden)
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
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if dto.RelationType == model.RelationTypeSchedule && dto.RelationSubType == nil {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}
	if dto.RelationType != model.RelationTypeSchedule {
		if dto.RelationSubType != nil || dto.LagMinutes != nil {
			_ = c.Error(errs.ErrBadRequest)
			c.Status(http.StatusBadRequest)
			return
		}
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var result *model.IssueRelation
	var recomputed []int64
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		recomputed = nil
		if !ic.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
			return errs.ErrForbidden
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
				return errs.ErrCycle
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
		if err != nil {
			return err
		}
		switch dto.RelationType {
		case model.RelationTypeHierarchy:
			if err := ic.hierarchy.RejectScheduleCycle(ctx, idProject, []int64{to}); err != nil {
				return err
			}
			recomputed, err = ic.hierarchy.RecomputeFrom(ctx, []int64{from})
			return err
		case model.RelationTypeSchedule:
			if err := ic.hierarchy.RejectScheduleOnOwnBranch(ctx, from, to); err != nil {
				return err
			}
			return ic.hierarchy.RejectScheduleCycle(ctx, idProject, []int64{from})
		}
		return nil
	})

	if err == errs.ErrForbidden {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if err == errs.ErrCycle {
		_ = c.Error(errs.ErrCycle)
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

	ic.broadcastRelationCreated(ctx, idProject, user.IdUser, views)
	agent.BroadcastIssueNotices(ctx, ic.notifier, ic.issueRepo, ic.projectRepo, recomputed, notify.ActionUpdate)

	c.JSON(http.StatusOK, views)
}

// Notify other project members over WebSocket for incremental client updates.
func (ic *IssueRelationController) broadcastRelationCreated(ctx context.Context, idProject, idActor int64, views []*model.ReadIssueRelationRes) {
	members, err := ic.projectRepo.LoadProjectsMembers(ctx, []int64{idProject})
	if err != nil {
		return
	}
	for _, member := range members {
		if member.IdUser == idActor {
			continue
		}
		ic.notifier.Send(&notify.Notice{
			IdUser:  member.IdUser,
			Subject: notify.SubjectRelation,
			Action:  notify.ActionCreate,
			Payload: views,
		})
	}
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

	var recomputed []int64
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		recomputed = nil
		if !ic.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
			return errs.ErrForbidden
		}
		relation, err := ic.relationRepo.LoadRelation(ctx, idRelation)
		if err != nil {
			return err
		}
		if err := ic.relationRepo.DeleteRelation(ctx, idRelation, idProject); err != nil {
			return err
		}
		if relation == nil || relation.IdProject != idProject || relation.RelationType != model.RelationTypeHierarchy {
			return nil
		}
		recomputed, err = ic.hierarchy.RecomputeFrom(ctx, []int64{relation.IdIssueFrom})
		return err
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

	agent.BroadcastIssueNotices(ctx, ic.notifier, ic.issueRepo, ic.projectRepo, recomputed, notify.ActionUpdate)

	c.Status(http.StatusOK)
}

func (ic *IssueRelationController) UpdateRelation(c *gin.Context) {
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

	if !ic.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
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

	relation, err := ic.relationRepo.LoadRelation(ctx, idRelation)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if relation == nil {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if relation.IdProject != idProject {
		// Relation belongs to another project — do not leak its existence.
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	if relation.RelationType != model.RelationTypeSchedule {
		_ = c.Error(fmt.Errorf("only schedule relations can be updated"))
		c.Status(http.StatusBadRequest)
		return
	}

	updated, err := ic.relationRepo.UpdateRelation(ctx, idRelation, dto.LagMinutes)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, updated)
}

type setParentRes struct {
	Relations         []*model.ReadIssueRelationRes `json:"relations"`
	IdRelationRemoved *int64                        `json:"idRelationRemoved"`
}

// SetParent swaps the hierarchy relation in one transaction, so an indent or outdent
// can never leave the task without a parent halfway through.
func (ic *IssueRelationController) SetParent(c *gin.Context) {
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
	var dto model.SetParentReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if dto.IdParent != nil && *dto.IdParent == idIssuePublic {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var created, remainingRelation *model.IssueRelation
	var remaining *model.Issue
	var removed *int64
	var recomputed []int64
	err = extctx.RunInTx(ctx, ic.pool, func(ctx context.Context) error {
		created, remainingRelation, remaining, removed, recomputed = nil, nil, nil, nil, nil
		if !ic.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
			return errs.ErrForbidden
		}
		child, err := ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &idProject, IdIssuePublic: &idIssuePublic})
		if err != nil {
			return err
		}
		var idParent int64
		var parent *model.Issue
		if dto.IdParent != nil {
			parent, err = ic.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdProject: &idProject, IdIssuePublic: dto.IdParent})
			if err != nil {
				return err
			}
			idParent = parent.IdIssue
		}
		idOldRelation, idOldParent, err := ic.hierarchyRepo.LoadParentRelation(ctx, child.IdIssue)
		if err != nil {
			return err
		}
		if idOldParent == idParent {
			return nil
		}
		var touched []int64
		if idOldRelation != 0 {
			if err := ic.relationRepo.DeleteRelation(ctx, idOldRelation, idProject); err != nil {
				return err
			}
			removed = &idOldRelation
			touched = append(touched, idOldParent)
		}
		if idParent != 0 {
			cycle, err := ic.relationRepo.HasCycle(ctx, model.RelationTypeHierarchy, idParent, child.IdIssue)
			if err != nil {
				return err
			}
			if cycle {
				return errs.ErrCycle
			}
			if dto.RemainingWork != nil && !parent.IsSummary && parent.Estimated > 0 {
				remaining, remainingRelation, err = ic.insertRemainingWork(ctx, parent, dto.RemainingWork, user.IdUser)
				if err != nil {
					return err
				}
			}
			created, err = ic.relationRepo.InsertRelation(ctx, &model.IssueRelation{
				IdProject:    idProject,
				IdIssueFrom:  idParent,
				IdIssueTo:    child.IdIssue,
				RelationType: model.RelationTypeHierarchy,
				CreatedBy:    user.IdUser,
			})
			if err != nil {
				return err
			}
			if err := ic.hierarchy.RejectScheduleCycle(ctx, idProject, []int64{child.IdIssue}); err != nil {
				return err
			}
			touched = append(touched, idParent)
		}
		recomputed, err = ic.hierarchy.RecomputeFrom(ctx, touched)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	}
	var appErr *errs.Error
	if err != nil && errs.As(err, &appErr) {
		_ = c.Error(appErr)
		c.Status(appErr.HttpStatus())
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	res := setParentRes{Relations: []*model.ReadIssueRelationRes{}, IdRelationRemoved: removed}
	for _, relation := range []*model.IssueRelation{remainingRelation, created} {
		if relation == nil {
			continue
		}
		views, err := ic.relationRepo.LoadRelationById(ctx, relation.IdIssueRelation)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusInternalServerError)
			return
		}
		res.Relations = append(res.Relations, views...)
		ic.broadcastRelationCreated(ctx, idProject, user.IdUser, views)
	}
	if remaining != nil {
		agent.BroadcastIssueNotices(ctx, ic.notifier, ic.issueRepo, ic.projectRepo, []int64{remaining.IdIssue}, notify.ActionCreate)
	}
	agent.BroadcastIssueNotices(ctx, ic.notifier, ic.issueRepo, ic.projectRepo, recomputed, notify.ActionUpdate)

	c.JSON(http.StatusOK, res)
}

func (ic *IssueRelationController) insertRemainingWork(ctx context.Context, parent *model.Issue, req *model.RemainingWorkReq, idUser int64) (*model.Issue, *model.IssueRelation, error) {
	child, err := ic.issueRepo.InsertIssue(ctx, &model.Issue{
		IdProject:   parent.IdProject,
		IdState:     parent.IdState,
		IdSeverity:  parent.IdSeverity,
		IdIssueType: parent.IdIssueType,
		Title:       req.Title,
		Description: req.Description,
		Estimated:   parent.Estimated,
		ScheduledAt: parent.ScheduledAt,
		CreateBy:    idUser,
		UpdateBy:    idUser,
	})
	if err != nil {
		return nil, nil, err
	}
	if err := ic.valueRepo.CopyRequiredValues(ctx, parent.IdIssue, child.IdIssue); err != nil {
		return nil, nil, err
	}
	relation, err := ic.relationRepo.InsertRelation(ctx, &model.IssueRelation{
		IdProject:    parent.IdProject,
		IdIssueFrom:  parent.IdIssue,
		IdIssueTo:    child.IdIssue,
		RelationType: model.RelationTypeHierarchy,
		CreatedBy:    idUser,
	})
	if err != nil {
		return nil, nil, err
	}
	return child, relation, nil
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
