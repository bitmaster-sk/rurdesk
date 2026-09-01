package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/ai"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
)

const rateLimitKeyPrefix = "ai:pb:ratelimit:"
const rateLimitTTL = 30 * time.Second

// ProjectBuilderController handles AI-assisted project backlog generation and acceptance.
type ProjectBuilderController struct {
	pool         *pgxpool.Pool
	aiProvider   ai.Provider
	cache        *redis.Client
	issueRepo    *repository.IssueRepository
	relationRepo *repository.IssueRelationRepository
	acl          *service.AclService
	notifier     *notify.Notifier
}

// NewProjectBuilderController creates a new ProjectBuilderController.
func NewProjectBuilderController(
	pool *pgxpool.Pool,
	aiProvider ai.Provider,
	cache *redis.Client,
	issueRepo *repository.IssueRepository,
	relationRepo *repository.IssueRelationRepository,
	acl *service.AclService,
	notifier *notify.Notifier,
) *ProjectBuilderController {
	return &ProjectBuilderController{
		pool:         pool,
		aiProvider:   aiProvider,
		cache:        cache,
		issueRepo:    issueRepo,
		relationRepo: relationRepo,
		acl:          acl,
		notifier:     notifier,
	}
}

// Generate calls the AI provider to generate a staged backlog for the project.
func (pc *ProjectBuilderController) Generate(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var req model.ProjectBuilderGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	if len(req.Description) > viper.GetInt("PROJECT_BUILDER_DESCRIPTION_MAX_LENGTH") {
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanCreateIssue(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// Rate limit: 1 call per user per 30 seconds.
	rateLimitKey := fmt.Sprintf("%s%d", rateLimitKeyPrefix, user.IdUser)
	set, err := pc.cache.SetNX(ctx, rateLimitKey, "1", rateLimitTTL).Result()
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !set {
		ttl, _ := pc.cache.TTL(ctx, rateLimitKey).Result()
		retryAfter := int(ttl.Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		c.Header("Retry-After", strconv.Itoa(retryAfter))
		_ = c.Error(errs.ErrRateLimited)
		c.JSON(errs.ErrRateLimited.HttpStatus(), errs.ErrRateLimited)
		return
	}

	aiModel := viper.GetString("AI_MODEL")
	if aiModel == "" {
		_ = c.Error(errs.ErrAiNotConfigured)
		c.JSON(errs.ErrAiNotConfigured.HttpStatus(), errs.ErrAiNotConfigured)
		return
	}
	completionReq := ai.CompletionReq{
		Model:     aiModel,
		Messages:  ai.BuildProjectBuilderPrompt(req.Description),
		Tools:     ai.ProjectBuilderTools(),
		MaxTokens: 32768,
	}

	res, err := pc.aiProvider.Complete(ctx, completionReq)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("AI provider error")
		_ = c.Error(err)
		c.JSON(errs.ErrAiUnavailable.HttpStatus(), errs.ErrAiUnavailable)
		return
	}

	extctx.GetLogger(ctx).Info().RawJSON("tool_use_input", res.ToolUseInput).Msg("AI raw response")

	issues, summary, err := ai.ParseProjectResponse(res)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Msg("AI response parsing error")
		_ = c.Error(err)
		c.JSON(errs.ErrAiInvalidResponse.HttpStatus(), errs.ErrAiInvalidResponse)
		return
	}

	for i := range issues {
		issues[i].IdState = req.IdState
		issues[i].IdSeverity = req.IdSeverity
	}

	c.JSON(http.StatusOK, model.ProjectBuilderGenerateRes{
		Issues:  issues,
		Summary: summary,
	})
}

// Accept saves the staged backlog to the database in a single transaction.
func (pc *ProjectBuilderController) Accept(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var req model.ProjectBuilderAcceptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanCreateIssue(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// In-payload cycle detection before any DB write.
	if err := detectCycles(req.Issues); err != nil {
		_ = c.Error(err)
		c.JSON(errs.ErrCycle.HttpStatus(), errs.ErrCycle)
		return
	}

	if err := validateRefs(req.Issues); err != nil {
		_ = c.Error(errs.ErrInvalidProjectBuilderRefs.WithMessage(err.Error()))
		c.Status(http.StatusUnprocessableEntity)
		return
	}

	var result []*model.Issue
	txErr := extctx.RunInTx(ctx, pc.pool, func(ctx context.Context) error {
		issuesToInsert := make([]model.Issue, len(req.Issues))
		for i, pbIssue := range req.Issues {
			issuesToInsert[i] = model.Issue{
				IdProject:   idProject,
				IdState:     pbIssue.IdState,
				IdSeverity:  pbIssue.IdSeverity,
				Title:       pbIssue.Title,
				Description: pbIssue.Description,
				Estimated:   pbIssue.EstimatedMinutes,
				CreateBy:    user.IdUser,
				UpdateBy:    user.IdUser,
			}
		}

		inserted, err := pc.issueRepo.BulkInsertIssues(ctx, issuesToInsert)
		if err != nil {
			return err
		}

		refToIdIssue := make(map[string]int64, len(inserted))
		var idsIssue []int64
		for i, iss := range inserted {
			refToIdIssue[req.Issues[i].Ref] = iss.IdIssue
			idsIssue = append(idsIssue, iss.IdIssue)
		}

		var relations []model.IssueRelation

		for _, pbIssue := range req.Issues {
			// Hierarchy relation: from = parent, to = child.
			if pbIssue.HierarchyParentRef != "" {
				parentId, ok := refToIdIssue[pbIssue.HierarchyParentRef]
				if !ok {
					continue
				}
				childId := refToIdIssue[pbIssue.Ref]
				relations = append(relations, model.IssueRelation{
					IdProject:    idProject,
					IdIssueFrom:  parentId,
					IdIssueTo:    childId,
					RelationType: model.RelationTypeHierarchy,
					CreatedBy:    user.IdUser,
				})
			}

			for _, sr := range pbIssue.ScheduleRelations {
				toId, ok := refToIdIssue[sr.Ref]
				if !ok {
					continue
				}
				subType := sr.Type
				relations = append(relations, model.IssueRelation{
					IdProject:       idProject,
					IdIssueFrom:     refToIdIssue[pbIssue.Ref],
					IdIssueTo:       toId,
					RelationType:    model.RelationTypeSchedule,
					RelationSubType: &subType,
					CreatedBy:       user.IdUser,
				})
			}
		}

		if len(relations) > 0 {
			if err := pc.relationRepo.BulkInsertRelations(ctx, relations); err != nil {
				return err
			}
		}

		filter := &model.LoadIssuesFilter{IdProject: idProject, IdsIssue: idsIssue}
		result, err = pc.issueRepo.LoadIssuesByIds(ctx, filter)
		return err
	})

	if txErr != nil {
		_ = c.Error(txErr)
		c.Status(http.StatusInternalServerError)
		return
	}

	for _, iss := range result {
		pc.notifier.Send <- &notify.Notice{
			IdUser:  user.IdUser,
			Subject: notify.SubjectIssue,
			Action:  notify.ActionCreate,
			Payload: iss.IdIssuePublic,
		}
	}

	c.JSON(http.StatusOK, model.ProjectBuilderAcceptRes{Issues: result})
}

// detectCycles DFS-checks the payload's hierarchy and schedule relations for cycles.
func detectCycles(issues []model.ProjectBuilderIssue) error {
	adj := make(map[string][]string, len(issues))
	for _, iss := range issues {
		if iss.HierarchyParentRef != "" {
			// Edge points child → parent for cycle-check purposes.
			adj[iss.Ref] = append(adj[iss.Ref], iss.HierarchyParentRef)
		}
		for _, sr := range iss.ScheduleRelations {
			adj[iss.Ref] = append(adj[iss.Ref], sr.Ref)
		}
	}

	visited := make(map[string]bool)
	inStack := make(map[string]bool)

	var dfs func(ref string) bool
	dfs = func(ref string) bool {
		if inStack[ref] {
			return true // cycle
		}
		if visited[ref] {
			return false
		}
		visited[ref] = true
		inStack[ref] = true
		for _, neighbor := range adj[ref] {
			if dfs(neighbor) {
				return true
			}
		}
		inStack[ref] = false
		return false
	}

	for _, iss := range issues {
		if !visited[iss.Ref] {
			if dfs(iss.Ref) {
				return errs.ErrCycle
			}
		}
	}
	return nil
}

// validateRefs checks that every relation ref in the payload resolves to a known ref.
func validateRefs(issues []model.ProjectBuilderIssue) error {
	refSet := make(map[string]bool, len(issues))
	for _, iss := range issues {
		refSet[iss.Ref] = true
	}
	for _, iss := range issues {
		if iss.HierarchyParentRef != "" && !refSet[iss.HierarchyParentRef] {
			return fmt.Errorf("dangling hierarchy_parent_ref %q in issue %q", iss.HierarchyParentRef, iss.Ref)
		}
		for _, sr := range iss.ScheduleRelations {
			if !refSet[sr.Ref] {
				return fmt.Errorf("dangling schedule relation ref %q in issue %q", sr.Ref, iss.Ref)
			}
		}
	}
	return nil
}
