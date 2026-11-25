package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bitmaster-sk/rurdesk/api/internal/attachmenttext"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type MessageController struct {
	messageRepo       *repository.MessageRepository
	teamRepo          *repository.TeamRepository
	projectRepo       *repository.ProjectRepository
	userRepo          *repository.UserRepository
	issueRepo         *repository.IssueRepository
	agentRunRepo      *repository.AgentRunRepository
	participantRepo   *repository.IssueParticipantRepository
	notifier          notify.INotifier
	acl               *service.AclService
	messageAccess     *service.MessageAccessService
	messageAnchorSvc  *service.MessageAnchorService
	attachmentLinkSvc *service.AttachmentLinkService
	notifSvc          *service.NotificationService
	pool              *pgxpool.Pool
}

func NewMessageController(
	mr *repository.MessageRepository,
	tr *repository.TeamRepository,
	pr *repository.ProjectRepository,
	ur *repository.UserRepository,
	ir *repository.IssueRepository,
	agentRunRepo *repository.AgentRunRepository,
	nf notify.INotifier,
	acl *service.AclService,
	messageAccess *service.MessageAccessService,
	messageAnchorSvc *service.MessageAnchorService,
	attachmentLinkSvc *service.AttachmentLinkService,
	notifSvc *service.NotificationService,
	participantRepo *repository.IssueParticipantRepository,
	pool *pgxpool.Pool,
) *MessageController {
	return &MessageController{
		messageRepo:       mr,
		teamRepo:          tr,
		projectRepo:       pr,
		userRepo:          ur,
		issueRepo:         ir,
		agentRunRepo:      agentRunRepo,
		notifier:          nf,
		acl:               acl,
		messageAccess:     messageAccess,
		messageAnchorSvc:  messageAnchorSvc,
		attachmentLinkSvc: attachmentLinkSvc,
		notifSvc:          notifSvc,
		participantRepo:   participantRepo,
		pool:              pool,
	}
}

func (mc *MessageController) GetMessages(c *gin.Context) {
	idRecipient, err := strconv.ParseInt(c.Query("idRecipient"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idType, err := strconv.ParseInt(c.Query("idMessageRecipientType"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	recipientType := model.MessageRecipientType(idType)

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if accessErr := mc.messageAccess.CheckContextAccess(ctx, user.IdUser, recipientType, idRecipient); accessErr != nil {
		ResponseErr(c, accessErr)
		return
	}

	var msgs []*model.Message

	switch recipientType {
	case model.TeammateRecipientType:
		msgs, err = mc.messageRepo.LoadTeammateMessages(ctx, idRecipient, user.IdUser, nil)
	case model.TeamRecipientType:
		msgs, err = mc.messageRepo.LoadTeamMessages(ctx, []int64{idRecipient}, user.IdUser, nil)
	case model.ProjectRecipientType:
		msgs, err = mc.messageRepo.LoadProjectMessages(ctx, []int64{idRecipient}, user.IdUser, nil)
	case model.IssueRecipientType:
		msgs, err = mc.messageAnchorSvc.LoadIssueMessages(ctx, idRecipient, user.IdUser)
	}

	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	if len(msgs) == 0 {
		c.JSON(http.StatusOK, []model.Message{})
		return
	}
	c.JSON(http.StatusOK, msgs)
}

func (mc *MessageController) GetUnreadMessages(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	msgs := []*model.Message{}
	read := false

	teams, err := mc.teamRepo.LoadTeams(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	idsTeam := make([]int64, 0, len(teams))
	for _, t := range teams {
		idsTeam = append(idsTeam, t.IdTeam)
	}

	projects, err := mc.acl.LoadVisibleProjects(ctx, user.IdUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	idsProject := make([]int64, 0, len(projects))
	for _, p := range projects {
		idsProject = append(idsProject, p.IdProject)
	}

	chunk, err := mc.messageRepo.LoadTeammateMessages(ctx, 0, user.IdUser, &read)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	msgs = append(msgs, chunk...)

	chunk, err = mc.messageRepo.LoadTeamMessages(ctx, idsTeam, user.IdUser, &read)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	msgs = append(msgs, chunk...)

	chunk, err = mc.messageRepo.LoadProjectMessages(ctx, idsProject, user.IdUser, &read)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	msgs = append(msgs, chunk...)

	c.JSON(http.StatusOK, msgs)
}

func (mc *MessageController) CreateMessage(c *gin.Context) {
	var dto model.CreateMessageReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	if err := dto.Validate(); err != nil {
		_ = c.Error(errs.ErrValidation.WithMessage(err.Error()))
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	// Agents may not call POST /message on an issue while the run is paused on the
	// user (awaiting_approval or awaiting_input) — plan submissions go through
	// submit_plan, clarifications through request_clarification. A raw
	// post_issue_message in a paused phase indicates a misbehaving model.
	if dto.IdMessageRecipientType == model.IssueRecipientType && user.IsAgent {
		activeRun, err := mc.agentRunRepo.LoadActiveByIssue(ctx, dto.IdRecipient)
		if err == nil && activeRun != nil && activeRun.IdUserAgent == user.IdUser &&
			(activeRun.Phase == constants.PhaseAwaitingApproval || activeRun.Phase == constants.PhaseAwaitingInput) {
			_ = c.Error(errs.ErrAgentPostWhileRunPaused)
			c.Status(http.StatusConflict)
			return
		}
	}

	var msg *model.Message
	mentionedIds := map[int64]bool{}

	err := extctx.RunInTx(ctx, mc.pool, func(ctx context.Context) error {
		if accessErr := mc.messageAccess.CheckContextAccess(ctx, user.IdUser, dto.IdMessageRecipientType, dto.IdRecipient); accessErr != nil {
			return accessErr
		}
		var err error
		switch dto.IdMessageRecipientType {
		case model.TeammateRecipientType:
			active, activeErr := mc.userRepo.IsActiveUser(ctx, dto.IdRecipient)
			if activeErr != nil {
				return activeErr
			}
			if !active {
				return errs.ErrUserDisabled
			}
			msg, err = mc.messageRepo.InsertTeammateMessage(ctx, dto.Message, &user, dto.IdRecipient)

		case model.TeamRecipientType:
			msg, err = mc.messageRepo.InsertTeamMessage(ctx, dto.Message, &user, dto.IdRecipient)

		case model.ProjectRecipientType:
			msg, err = mc.messageRepo.InsertProjectMessage(ctx, dto.Message, &user, dto.IdRecipient)

		case model.IssueRecipientType:
			var anchor *model.MessageAnchor
			if dto.IdParentMessage != nil {
				anchor = &model.MessageAnchor{
					IdParentMessage: *dto.IdParentMessage,
					AnchorLineStart: *dto.AnchorLineStart,
					AnchorLineEnd:   *dto.AnchorLineEnd,
				}
				if dto.AnchorQuote != nil {
					quote := strings.TrimSpace(*dto.AnchorQuote)
					anchor.Quote = &quote
				}
			}
			if dto.IdThreadMessage != nil {
				anchor = &model.MessageAnchor{IdThreadMessage: dto.IdThreadMessage}
			}
			msg, err = mc.messageAnchorSvc.CreateIssueMessage(ctx, dto.Message, &user, dto.IdRecipient, anchor)
			if err != nil {
				return err
			}
			// Auto-add comment author as participant (source="comment").
			if err := mc.participantRepo.Add(ctx, dto.IdRecipient, user.IdUser, "comment", &user.IdUser); err != nil {
				return fmt.Errorf("adding comment author as participant: %w", err)
			}
			// Auto-add @mentioned project members as participants (source="mention").
			// Mentions are detected from the token format @[name](user:<id>).
			mentionIds := parseMentionUserIds(dto.Message)
			if len(mentionIds) > 0 {
				project, projectErr := mc.projectRepo.LoadProjectByIssue(ctx, dto.IdRecipient)
				if projectErr != nil {
					return projectErr
				}
				members, membersErr := mc.projectRepo.LoadProjectsMembers(ctx, []int64{project.IdProject})
				if membersErr != nil {
					return fmt.Errorf("loading project members for mention resolution: %w", membersErr)
				}
				memberSet := make(map[int64]bool, len(members))
				for _, member := range members {
					memberSet[member.IdUser] = true
				}
				for _, idUser := range mentionIds {
					if !memberSet[idUser] {
						continue // mention of a non-member is ignored (cannot access the issue)
					}
					if err := mc.participantRepo.Add(ctx, dto.IdRecipient, idUser, "mention", &user.IdUser); err != nil {
						return fmt.Errorf("adding mentioned user %d as participant: %w", idUser, err)
					}
					mentionedIds[idUser] = true
				}
			}
		}
		if err != nil {
			return err
		}
		return mc.attachmentLinkSvc.LinkMessageAttachments(ctx, msg)
	})
	if errors.Is(err, service.ErrAnchorWrongThread) || errors.Is(err, service.ErrAnchorNestedParent) ||
		errors.Is(err, service.ErrAnchorLineOutOfRange) || errors.Is(err, service.ErrAnchorInvalidThread) {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	if err != nil {
		ResponseErr(c, err)
		return
	}

	// Send notifications outside the transaction
	source := ""
	if isAgent, err := mc.userRepo.IsAgentUser(ctx, user.IdUser); err == nil && isAgent {
		source = "agent"
	}

	switch dto.IdMessageRecipientType {
	case model.TeammateRecipientType:
		mc.notifier.Send(&notify.Notice{
			IdUser:  dto.IdRecipient,
			Subject: notify.SubjectMessage,
			Action:  notify.ActionCreate,
			Payload: msg,
			Source:  source,
		})
	case model.TeamRecipientType:
		users, err := mc.teamRepo.LoadTeamsMembers(ctx, []int64{dto.IdRecipient})
		if err == nil {
			for _, u := range users {
				if u.IdUser == user.IdUser {
					continue
				}
				mc.notifier.Send(&notify.Notice{
					IdUser:  u.IdUser,
					Subject: notify.SubjectMessage,
					Action:  notify.ActionCreate,
					Payload: msg,
					Source:  source,
				})
			}
		}
	case model.ProjectRecipientType:
		users, err := mc.projectRepo.LoadProjectsMembers(ctx, []int64{dto.IdRecipient})
		if err == nil {
			for _, u := range users {
				if u.IdUser == user.IdUser {
					continue
				}
				mc.notifier.Send(&notify.Notice{
					IdUser:  u.IdUser,
					Subject: notify.SubjectMessage,
					Action:  notify.ActionCreate,
					Payload: msg,
					Source:  source,
				})
			}
		}
	case model.IssueRecipientType:
		project, err := mc.projectRepo.LoadProjectByIssue(ctx, dto.IdRecipient)
		if err == nil {
			// Load the issue to populate RefTitle/RefPublicId — without these the
			// frontend can't render a link to it.
			var (
				refTitle    string
				refPublicId *int64
			)
			if loadedIssue, issErr := mc.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &dto.IdRecipient}); issErr == nil {
				refTitle = loadedIssue.Title
				idIssuePublic := loadedIssue.IdIssuePublic
				refPublicId = &idIssuePublic
			}

			// WS realtime delivery: send to EVERY project member so the activity
			// feed re-renders for everyone in the project.
			members, membersErr := mc.projectRepo.LoadProjectsMembers(ctx, []int64{project.IdProject})
			if membersErr == nil {
				for _, member := range members {
					if member.IdUser == user.IdUser {
						continue
					}
					mc.notifier.Send(&notify.Notice{
						IdUser:  member.IdUser,
						Subject: notify.SubjectMessage,
						Action:  notify.ActionCreate,
						Payload: msg,
						Source:  source,
					})
				}
			}

			// Persistent notifications: send ONLY to notifiable participants
			// (enabled=true, not an agent), skipping the comment author. Runs
			// regardless of whether the WS-only members load above succeeded.
			notifiableIds, notifiableErr := mc.participantRepo.NotifiableUserIds(ctx, dto.IdRecipient)
			if notifiableErr != nil {
				log.Warn().Err(notifiableErr).Int64("idIssue", dto.IdRecipient).Msg("CreateMessage: failed to load notifiable participant ids")
			} else {
				idProject := project.IdProject
				for _, idRecipient := range notifiableIds {
					if idRecipient == user.IdUser {
						continue
					}
					notifType := constants.NotificationTypeComment
					if mentionedIds[idRecipient] {
						notifType = constants.NotificationTypeMention
					}
					if notifErr := mc.notifSvc.Notify(ctx, &model.CreateNotificationReq{
						IdUser:              idRecipient,
						Type:                notifType,
						IdProject:           &idProject,
						ProjectName:         project.Name,
						ProjectColor:        project.Color,
						ActorName:           user.Name,
						ActorAvatarBg:       user.ColorAvatarBg,
						ActorAvatarInitials: user.AvatarInitials,
						RefType:             constants.NotificationRefTypeIssue,
						RefId:               strconv.FormatInt(dto.IdRecipient, 10),
						RefTitle:            refTitle,
						RefPublicId:         refPublicId,
						Body:                truncate(stripMentionTokens(attachmenttext.StripLinks(dto.Message)), 200),
						Source:              source,
					}); notifErr != nil {
						log.Warn().Err(notifErr).Int64("idUser", idRecipient).Int64("idIssue", dto.IdRecipient).Msg("CreateMessage: failed to create notification")
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, msg)
}

func (mc *MessageController) SetReadMessages(c *gin.Context) {
	var dto model.SetReadMessageReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	// Validated before opening a transaction: an unknown type is the caller's
	// mistake, and falling through marked nothing read while answering 200.
	switch dto.IdMessageRecipientType {
	case model.TeammateRecipientType, model.TeamRecipientType, model.ProjectRecipientType:
	case model.IssueRecipientType:
		if accessErr := mc.messageAccess.CheckContextAccess(ctx, user.IdUser, dto.IdMessageRecipientType, dto.IdRecipient); accessErr != nil {
			ResponseErr(c, accessErr)
			return
		}
	default:
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
	}

	err := extctx.RunInTx(ctx, mc.pool, func(ctx context.Context) error {
		switch dto.IdMessageRecipientType {
		case model.TeammateRecipientType:
			return mc.messageRepo.InsertReadTeammatesMessages(ctx, dto.IdRecipient, user.IdUser)
		case model.TeamRecipientType:
			return mc.messageRepo.InsertReadTeamMessages(ctx, dto.IdRecipient, user.IdUser)
		case model.IssueRecipientType:
			return mc.messageRepo.InsertReadIssueMessages(ctx, dto.IdRecipient, user.IdUser)
		default:
			return mc.messageRepo.InsertReadProjectMessages(ctx, dto.IdRecipient, user.IdUser)
		}
	})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (mc *MessageController) UpdateMessage(c *gin.Context) {
	idMessage, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	var dto model.UpdateMessageReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var updatedMsg *model.Message
	err = extctx.RunInTx(ctx, mc.pool, func(ctx context.Context) error {
		var txErr error
		updatedMsg, txErr = mc.messageRepo.UpdateMessage(ctx, idMessage, user.IdUser, dto.Message)
		if txErr != nil || updatedMsg == nil {
			return txErr
		}
		updatedMsg.IdRecipient, updatedMsg.IdMessageRecipientType, txErr = mc.messageRepo.LoadMessageRecipientInfo(ctx, idMessage)
		if txErr != nil {
			return txErr
		}
		return mc.attachmentLinkSvc.LinkMessageAttachments(ctx, updatedMsg)
	})
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if updatedMsg == nil {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	idRecipient, recipientType := updatedMsg.IdRecipient, updatedMsg.IdMessageRecipientType

	updateSource := ""
	if isAgent, err := mc.userRepo.IsAgentUser(ctx, user.IdUser); err == nil && isAgent {
		updateSource = "agent"
	}

	switch recipientType {
	case model.TeammateRecipientType:
		mc.notifier.Send(&notify.Notice{
			IdUser:  idRecipient,
			Subject: notify.SubjectMessage,
			Action:  notify.ActionUpdate,
			Payload: updatedMsg,
			Source:  updateSource,
		})
	case model.TeamRecipientType:
		users, err := mc.teamRepo.LoadTeamsMembers(ctx, []int64{idRecipient})
		if err == nil {
			for _, u := range users {
				mc.notifier.Send(&notify.Notice{
					IdUser:  u.IdUser,
					Subject: notify.SubjectMessage,
					Action:  notify.ActionUpdate,
					Payload: updatedMsg,
					Source:  updateSource,
				})
			}
		}
	case model.ProjectRecipientType:
		users, err := mc.projectRepo.LoadProjectsMembers(ctx, []int64{idRecipient})
		if err == nil {
			for _, u := range users {
				mc.notifier.Send(&notify.Notice{
					IdUser:  u.IdUser,
					Subject: notify.SubjectMessage,
					Action:  notify.ActionUpdate,
					Payload: updatedMsg,
					Source:  updateSource,
				})
			}
		}
	case model.IssueRecipientType:
		project, err := mc.projectRepo.LoadProjectByIssue(ctx, idRecipient)
		if err == nil {
			users, err := mc.projectRepo.LoadProjectsMembers(ctx, []int64{project.IdProject})
			if err == nil {
				for _, u := range users {
					mc.notifier.Send(&notify.Notice{
						IdUser:  u.IdUser,
						Subject: notify.SubjectMessage,
						Action:  notify.ActionUpdate,
						Payload: updatedMsg,
						Source:  updateSource,
					})
				}
			}
		}
	}

	c.JSON(http.StatusOK, updatedMsg)
}

func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	return string([]rune(s)[:maxRunes]) + "..."
}
