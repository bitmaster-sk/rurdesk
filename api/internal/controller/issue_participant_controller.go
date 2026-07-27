package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// IssueParticipantController handles HTTP endpoints for issue participants:
// listing, manual-add (project-member only), and self-mute.
type IssueParticipantController struct {
	participantRepo *repository.IssueParticipantRepository
	issueRepo       *repository.IssueRepository
	projectRepo     *repository.ProjectRepository
	acl             *service.AclService
	notifier        *notify.Notifier
}

// NewIssueParticipantController wires all dependencies.
func NewIssueParticipantController(
	participantRepo *repository.IssueParticipantRepository,
	issueRepo *repository.IssueRepository,
	projectRepo *repository.ProjectRepository,
	acl *service.AclService,
	notifier *notify.Notifier,
) *IssueParticipantController {
	return &IssueParticipantController{
		participantRepo: participantRepo,
		issueRepo:       issueRepo,
		projectRepo:     projectRepo,
		acl:             acl,
		notifier:        notifier,
	}
}

// GetParticipants handles GET /project/:idProject/issue/:idIssuePublic/participant.
// Requires CanReadProject for the calling user.
func (pc *IssueParticipantController) GetParticipants(c *gin.Context) {
	idProject, idIssue, ok := pc.resolveIDs(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	list, err := pc.participantRepo.List(ctx, idIssue)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, list)
}

// AddParticipant handles POST /project/:idProject/issue/:idIssuePublic/participant.
// Requires CanUpdateIssue for the caller, and CanReadProject for the added user —
// otherwise an outsider could gain access to an issue via participation.
func (pc *IssueParticipantController) AddParticipant(c *gin.Context) {
	idProject, idIssue, ok := pc.resolveIDs(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanUpdateIssue(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	var dto model.AddParticipantReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	if !pc.acl.CanReadProject(ctx, dto.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	if err := pc.participantRepo.Add(ctx, idIssue, dto.IdUser, "manual", &user.IdUser); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	pc.broadcast(c, idProject, idIssue)
	c.Status(http.StatusOK)
}

// SetMyNotifications handles PATCH /project/:idProject/issue/:idIssuePublic/participant/notifications.
// Self-only: always changes the row for the calling user — the request body
// contains only { "enabled": bool }. There is no way to mute another user.
// Returns 404 when the calling user is not a participant of the issue.
func (pc *IssueParticipantController) SetMyNotifications(c *gin.Context) {
	idProject, idIssue, ok := pc.resolveIDs(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !pc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	var dto model.SetParticipantNotificationsReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	// Self-only: always use user.IdUser from the auth context, never an id from the
	// request body or URL.
	found, err := pc.participantRepo.SetNotifications(ctx, idIssue, user.IdUser, dto.Enabled)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !found {
		c.Status(http.StatusNotFound)
		return
	}

	pc.broadcast(c, idProject, idIssue)
	c.Status(http.StatusOK)
}

// resolveIDs parses :idProject/:idIssuePublic and resolves the public issue id to
// its internal id_issue. On failure it writes the error response and returns ok=false.
func (pc *IssueParticipantController) resolveIDs(c *gin.Context) (idProject int64, idIssue int64, ok bool) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return 0, 0, false
	}

	idIssuePublic, err := strconv.ParseInt(c.Param("idIssuePublic"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return 0, 0, false
	}

	ctx := c.Request.Context()
	issue, err := pc.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{
		IdProject:     &idProject,
		IdIssuePublic: &idIssuePublic,
	})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return 0, 0, false
	}

	return idProject, issue.IdIssue, true
}

// broadcast sends the updated participant list to all project members via
// WebSocket so their clients can refresh without polling.
func (pc *IssueParticipantController) broadcast(c *gin.Context, idProject, idIssue int64) {
	broadcastParticipants(c.Request.Context(), pc.notifier, pc.projectRepo, pc.participantRepo, idProject, idIssue)
}

// broadcastParticipants pushes an issue's participant list to every project
// member over WebSocket (SubjectIssueParticipant). Shared by the participant
// endpoints and the issue edit paths (assigning a user adds a participant).
// Best-effort: a load error is logged and skipped, never blocking the caller.
func broadcastParticipants(
	ctx context.Context,
	notifier *notify.Notifier,
	projectRepo *repository.ProjectRepository,
	participantRepo *repository.IssueParticipantRepository,
	idProject, idIssue int64,
) {
	members, err := projectRepo.LoadProjectsMembers(ctx, []int64{idProject})
	if err != nil || len(members) == 0 {
		return
	}

	idsUser := make([]int64, 0, len(members))
	for _, member := range members {
		idsUser = append(idsUser, member.IdUser)
	}

	list, err := participantRepo.List(ctx, idIssue)
	if err != nil {
		log.Warn().Err(err).Int64("idIssue", idIssue).Msg("broadcastParticipants: failed to load participant list; skipping WS broadcast")
		return
	}

	notifier.Send <- &notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectIssueParticipant,
		Action:  notify.ActionUpdate,
		Payload: gin.H{"idIssue": idIssue, "participants": list},
	}
}
