package controller

import (
	"context"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type ReleaseController struct {
	releaseRepo      *repository.ReleaseRepository
	releases         *service.ReleaseService
	acl              *service.AclService
	tagCtrl          *TagController
	agentRunRepo     *repository.AgentRunRepository
	agentTaskRepo    *repository.AgentTaskRepository
	issueRepo        *repository.IssueRepository
	projectRepo      *repository.ProjectRepository
	projectMemberSvc *service.ProjectMemberService
	thinking         *agent.ThinkingService
	dispatcher       *agent.Dispatcher
	notifier         *notify.Notifier
}

type ReleaseControllerDeps struct {
	ReleaseRepo      *repository.ReleaseRepository
	Releases         *service.ReleaseService
	Acl              *service.AclService
	TagCtrl          *TagController
	AgentRunRepo     *repository.AgentRunRepository
	AgentTaskRepo    *repository.AgentTaskRepository
	IssueRepo        *repository.IssueRepository
	ProjectRepo      *repository.ProjectRepository
	ProjectMemberSvc *service.ProjectMemberService
	Thinking         *agent.ThinkingService
	Dispatcher       *agent.Dispatcher
	Notifier         *notify.Notifier
}

func NewReleaseController(deps ReleaseControllerDeps) *ReleaseController {
	return &ReleaseController{
		releaseRepo:      deps.ReleaseRepo,
		releases:         deps.Releases,
		acl:              deps.Acl,
		tagCtrl:          deps.TagCtrl,
		agentRunRepo:     deps.AgentRunRepo,
		agentTaskRepo:    deps.AgentTaskRepo,
		issueRepo:        deps.IssueRepo,
		projectRepo:      deps.ProjectRepo,
		projectMemberSvc: deps.ProjectMemberSvc,
		thinking:         deps.Thinking,
		dispatcher:       deps.Dispatcher,
		notifier:         deps.Notifier,
	}
}

func (rc *ReleaseController) GetReleases(c *gin.Context) {
	idProject, ok := parseIdParam(c, "idProject")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	releases, err := rc.releaseRepo.LoadReleases(ctx, idProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, releases)
}

func (rc *ReleaseController) GetDraft(c *gin.Context) {
	idProject, ok := parseIdParam(c, "idProject")
	if !ok {
		return
	}
	idTag, err := strconv.ParseInt(c.Query("idTag"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage("idTag is required"))
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	draft, err := rc.releases.LoadDraft(ctx, idProject, idTag)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, draft)
}

func (rc *ReleaseController) GetRelease(c *gin.Context) {
	idProject, idRelease, ok := parseReleaseParams(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	rc.respondRelease(c, idProject, idRelease)
}

func (rc *ReleaseController) CreateRelease(c *gin.Context) {
	idProject, ok := parseIdParam(c, "idProject")
	if !ok {
		return
	}
	var dto model.CreateReleaseReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanUpdateProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	created, err := rc.releases.CreateDraft(ctx, user, idProject, dto)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	rc.sendRelease(ctx, idProject, created.IdRelease, notify.ActionCreate)
	rc.respondRelease(c, idProject, created.IdRelease)
}

func (rc *ReleaseController) UpdateRelease(c *gin.Context) {
	idProject, idRelease, ok := parseReleaseParams(c)
	if !ok {
		return
	}
	var dto model.UpdateReleaseReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanUpdateProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	if _, err := rc.releases.UpdateDraft(ctx, idProject, idRelease, dto); err != nil {
		ResponseErr(c, err)
		return
	}
	rc.sendRelease(ctx, idProject, idRelease, notify.ActionUpdate)
	rc.respondRelease(c, idProject, idRelease)
}

func (rc *ReleaseController) StartAgent(c *gin.Context) {
	idProject, idRelease, ok := parseReleaseParams(c)
	if !ok {
		return
	}
	var dto model.StartReleaseAgentReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanUpdateProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	result, err := rc.releases.StartAgent(ctx, idProject, idRelease, dto)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	rc.broadcast(ctx, result)
	rc.sendRelease(ctx, idProject, idRelease, notify.ActionUpdate)
	rc.respondRelease(c, idProject, idRelease)
}

func (rc *ReleaseController) PublishRelease(c *gin.Context) {
	idProject, idRelease, ok := parseReleaseParams(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanUpdateProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	result, err := rc.releases.Publish(ctx, user, idProject, idRelease)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	rc.broadcast(ctx, result)
	rc.sendRelease(ctx, idProject, idRelease, notify.ActionUpdate)
	rc.respondRelease(c, idProject, idRelease)
}

func (rc *ReleaseController) UpdateNotes(c *gin.Context) {
	idProject, idRelease, ok := parseReleaseParams(c)
	if !ok {
		return
	}
	var req model.UpdateReleaseNotesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanUpdateProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	if _, err := rc.releases.UpdateReleasedNotes(ctx, idProject, idRelease, req.Notes); err != nil {
		ResponseErr(c, err)
		return
	}
	rc.sendRelease(ctx, idProject, idRelease, notify.ActionUpdate)
	rc.respondRelease(c, idProject, idRelease)
}

func (rc *ReleaseController) DeleteRelease(c *gin.Context) {
	idProject, idRelease, ok := parseReleaseParams(c)
	if !ok {
		return
	}
	var dto model.DeleteReleaseReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !rc.acl.CanUpdateProject(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	result, err := rc.releases.Delete(ctx, user, idProject, idRelease, dto)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	rc.broadcast(ctx, result)
	sendRelease(ctx, rc.notifier, rc.projectMemberSvc, idProject, result.Release, notify.ActionDelete)
	ResponseOk(c, result.Release)
}

func (rc *ReleaseController) respondRelease(c *gin.Context, idProject, idRelease int64) {
	release, err := rc.releaseRepo.LoadRelease(c.Request.Context(), idProject, idRelease)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, release)
}

func (rc *ReleaseController) broadcast(ctx context.Context, result *service.ReleaseResult) {
	rc.tagCtrl.BroadcastTag(ctx, result.RenamedTag, notify.ActionUpdate)
	rc.tagCtrl.BroadcastTag(ctx, result.CreatedTag, notify.ActionCreate)
	rc.tagCtrl.BroadcastTag(ctx, result.DeletedTag, notify.ActionDelete)
	for _, idIssue := range result.IdsIssueChanged {
		agent.BroadcastIssueUpdate(ctx, rc.notifier, rc.issueRepo, rc.projectRepo, idIssue)
	}
	if result.Run != nil {
		agent.BroadcastRunUpdate(ctx, rc.notifier, rc.projectRepo, rc.issueRepo, rc.agentRunRepo, rc.agentTaskRepo, result.Run)
	}
	if result.CancelledRun != nil {
		rc.thinking.Compact(ctx, result.IdsTaskStopped...)
		cancelled := result.CancelledRun
		go func() {
			_ = rc.dispatcher.DispatchCancelled(context.Background(), cancelled)
		}()
		agent.BroadcastRunUpdate(ctx, rc.notifier, rc.projectRepo, rc.issueRepo, rc.agentRunRepo, rc.agentTaskRepo, cancelled)
	}
}

func (rc *ReleaseController) sendRelease(ctx context.Context, idProject, idRelease int64, action notify.NoticeAction) {
	release, err := rc.releaseRepo.LoadRelease(ctx, idProject, idRelease)
	if err != nil {
		return
	}
	sendRelease(ctx, rc.notifier, rc.projectMemberSvc, idProject, release, action)
}

func BroadcastRelease(ctx context.Context, notifier *notify.Notifier, members *service.ProjectMemberService, releaseRepo *repository.ReleaseRepository, idProject, idRelease int64) {
	release, err := releaseRepo.LoadRelease(ctx, idProject, idRelease)
	if err != nil {
		return
	}
	sendRelease(ctx, notifier, members, idProject, release, notify.ActionUpdate)
}

func sendRelease(ctx context.Context, notifier *notify.Notifier, members *service.ProjectMemberService, idProject int64, payload any, action notify.NoticeAction) {
	if notifier == nil || payload == nil {
		return
	}
	idsUser, err := members.LoadMemberIds(ctx, idProject)
	if err != nil || len(idsUser) == 0 {
		return
	}
	notifier.Send <- &notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectRelease,
		Action:  action,
		Payload: payload,
	}
}

func parseReleaseParams(c *gin.Context) (int64, int64, bool) {
	idProject, ok := parseIdParam(c, "idProject")
	if !ok {
		return 0, 0, false
	}
	idRelease, ok := parseIdParam(c, "idRelease")
	return idProject, idRelease, ok
}

func parseIdParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return 0, false
	}
	return id, true
}
