package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
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
	messageRepo     *repository.MessageRepository
	teamRepo        *repository.TeamRepository
	projectRepo     *repository.ProjectRepository
	userRepo        *repository.UserRepository
	issueRepo       *repository.IssueRepository
	agentRunRepo    *repository.AgentRunRepository
	agentTaskRepo   *repository.AgentTaskRepository
	botGwRepo       *repository.BotGatewayRepository
	participantRepo *repository.IssueParticipantRepository
	dispatcher      *agent.Dispatcher
	notifier        *notify.Notifier
	acl             *service.AclService
	notifSvc        *service.NotificationService
	pool            *pgxpool.Pool
}

func NewMessageController(
	mr *repository.MessageRepository,
	tr *repository.TeamRepository,
	pr *repository.ProjectRepository,
	ur *repository.UserRepository,
	ir *repository.IssueRepository,
	nf *notify.Notifier,
	acl *service.AclService,
	notifSvc *service.NotificationService,
	participantRepo *repository.IssueParticipantRepository,
	pool *pgxpool.Pool,
) *MessageController {
	return &MessageController{
		messageRepo:     mr,
		teamRepo:        tr,
		projectRepo:     pr,
		userRepo:        ur,
		issueRepo:       ir,
		notifier:        nf,
		acl:             acl,
		notifSvc:        notifSvc,
		participantRepo: participantRepo,
		pool:            pool,
	}
}

func (mc *MessageController) WithAgentRun(
	agentRunRepo *repository.AgentRunRepository,
	agentTaskRepo *repository.AgentTaskRepository,
	botGwRepo *repository.BotGatewayRepository,
	dispatcher *agent.Dispatcher,
	notifier *notify.Notifier,
) *MessageController {
	mc.agentRunRepo = agentRunRepo
	mc.agentTaskRepo = agentTaskRepo
	mc.botGwRepo = botGwRepo
	mc.dispatcher = dispatcher
	mc.notifier = notifier
	return mc
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

	msgs := []*model.Message{}

	switch recipientType {
	case model.TeammateRecipientType:
		// DMs are open to all users — reading an empty conversation is harmless.
		msgs, err = mc.messageRepo.LoadTeammateMessages(ctx, idRecipient, user.IdUser, nil)

	case model.TeamRecipientType:
		if !mc.acl.CanReadTeam(ctx, user.IdUser, idRecipient) {
			_ = c.Error(errForbidden)
			c.Status(http.StatusForbidden)
			return
		}
		msgs, err = mc.messageRepo.LoadTeamMessages(ctx, []int64{idRecipient}, user.IdUser, nil)

	case model.ProjectRecipientType:
		if !mc.acl.CanReadProject(ctx, user.IdUser, idRecipient) {
			_ = c.Error(errForbidden)
			c.Status(http.StatusForbidden)
			return
		}
		msgs, err = mc.messageRepo.LoadProjectMessages(ctx, []int64{idRecipient}, user.IdUser, nil)

	case model.IssueRecipientType:
		project, e := mc.projectRepo.LoadProjectByIssue(ctx, idRecipient)
		if e != nil {
			_ = c.Error(e)
			c.Status(http.StatusInternalServerError)
			return
		}
		if !mc.acl.CanReadProject(ctx, user.IdUser, project.IdProject) {
			_ = c.Error(errForbidden)
			c.Status(http.StatusForbidden)
			return
		}
		msgs, err = mc.messageRepo.LoadIssueMessages(ctx, idRecipient, user.IdUser, nil)

	default:
		// Falling through returned an empty list, which reads as "this
		// conversation is empty" rather than "that recipient type does not exist".
		_ = c.Error(errs.ErrBadRequest)
		c.Status(http.StatusBadRequest)
		return
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

	projects, err := mc.projectRepo.LoadProjects(ctx, user.IdUser)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	// Bots may not call POST /message on an issue while the run is paused on the
	// user (awaiting_approval or awaiting_input) — plan submissions go through
	// submit_plan, clarifications through request_clarification. A raw
	// post_issue_message in a paused phase indicates a misbehaving model.
	if dto.IdMessageRecipientType == model.IssueRecipientType && mc.agentRunRepo != nil && user.IsBot {
		activeRun, err := mc.agentRunRepo.LoadActiveByIssue(ctx, dto.IdRecipient)
		if err == nil && activeRun != nil && activeRun.IdUserBot == user.IdUser &&
			(activeRun.Phase == constants.PhaseAwaitingApproval || activeRun.Phase == constants.PhaseAwaitingInput) {
			c.JSON(http.StatusConflict, gin.H{"error": "bot_post_while_run_paused"})
			return
		}
	}

	var msg *model.Message
	mentionedIds := map[int64]bool{}

	err := extctx.RunInTx(ctx, mc.pool, func(ctx context.Context) error {
		var err error
		switch dto.IdMessageRecipientType {
		case model.TeammateRecipientType:
			// DMs are open to all users — only require that the recipient exists.
			if _, lErr := mc.userRepo.LoadUser(ctx, dto.IdRecipient); lErr != nil {
				return errForbidden
			}
			msg, err = mc.messageRepo.InsertTeammateMessage(ctx, dto.Message, &user, dto.IdRecipient)

		case model.TeamRecipientType:
			if !mc.acl.CanReadTeam(ctx, user.IdUser, dto.IdRecipient) {
				return errForbidden
			}
			msg, err = mc.messageRepo.InsertTeamMessage(ctx, dto.Message, &user, dto.IdRecipient)

		case model.ProjectRecipientType:
			if !mc.acl.CanReadProject(ctx, user.IdUser, dto.IdRecipient) {
				return errForbidden
			}
			msg, err = mc.messageRepo.InsertProjectMessage(ctx, dto.Message, &user, dto.IdRecipient)

		case model.IssueRecipientType:
			project, e := mc.projectRepo.LoadProjectByIssue(ctx, dto.IdRecipient)
			if e != nil {
				return e
			}
			if !mc.acl.CanReadProject(ctx, user.IdUser, project.IdProject) {
				return errForbidden
			}
			var anchor *model.MessageAnchor
			if dto.IdParentMessage != nil {
				anchor = &model.MessageAnchor{
					IdParentMessage: *dto.IdParentMessage,
					AnchorLineStart: *dto.AnchorLineStart,
					AnchorLineEnd:   *dto.AnchorLineEnd,
				}
			}
			msg, err = mc.messageRepo.InsertIssueMessage(ctx, dto.Message, &user, dto.IdRecipient, anchor)
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
		return err
	})
	if err == errForbidden {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}
	if errors.Is(err, repository.ErrAnchorWrongThread) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// Send notifications outside the transaction
	source := ""
	if isBot, err := mc.userRepo.IsBotUser(ctx, user.IdUser); err == nil && isBot {
		source = "bot"
	}

	switch dto.IdMessageRecipientType {
	case model.TeammateRecipientType:
		mc.notifier.Send <- &notify.Notice{
			IdUser:  dto.IdRecipient,
			Subject: notify.SubjectMessage,
			Action:  notify.ActionCreate,
			Payload: msg,
			Source:  source,
		}
	case model.TeamRecipientType:
		users, err := mc.teamRepo.LoadTeamsMembers(ctx, []int64{dto.IdRecipient})
		if err == nil {
			for _, u := range users {
				if u.IdUser == user.IdUser {
					continue
				}
				mc.notifier.Send <- &notify.Notice{
					IdUser:  u.IdUser,
					Subject: notify.SubjectMessage,
					Action:  notify.ActionCreate,
					Payload: msg,
					Source:  source,
				}
			}
		}
	case model.ProjectRecipientType:
		users, err := mc.projectRepo.LoadProjectsMembers(ctx, []int64{dto.IdRecipient})
		if err == nil {
			for _, u := range users {
				if u.IdUser == user.IdUser {
					continue
				}
				mc.notifier.Send <- &notify.Notice{
					IdUser:  u.IdUser,
					Subject: notify.SubjectMessage,
					Action:  notify.ActionCreate,
					Payload: msg,
					Source:  source,
				}
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
					mc.notifier.Send <- &notify.Notice{
						IdUser:  member.IdUser,
						Subject: notify.SubjectMessage,
						Action:  notify.ActionCreate,
						Payload: msg,
						Source:  source,
					}
				}
			}

			// Persistent notifications: send ONLY to notifiable participants
			// (enabled=true, not a bot), skipping the comment author. Runs
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
					_ = mc.notifSvc.Notify(ctx, &model.CreateNotificationReq{
						IdUser:        idRecipient,
						Type:          notifType,
						IdProject:     &idProject,
						ProjectName:   project.Name,
						ProjectColor:  project.Color,
						ActorName:     user.Name,
						ActorAvatarBg: user.ColorAvatarBg,
						RefType:       constants.NotificationRefTypeIssue,
						RefId:         strconv.FormatInt(dto.IdRecipient, 10),
						RefTitle:      refTitle,
						RefPublicId:   refPublicId,
						Body:          truncate(stripMentionTokens(dto.Message), 200),
						Source:        source,
					})
				}
			}
		}
	}

	// Agent run hooks for issue messages
	if dto.IdMessageRecipientType == model.IssueRecipientType && mc.agentRunRepo != nil {
		mc.handleAgentRunHook(ctx, &user, msg, dto.IdRecipient)
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
		return txErr
	})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if updatedMsg == nil {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	idRecipient, recipientType, err := mc.messageRepo.LoadMessageRecipientInfo(ctx, idMessage)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	updatedMsg.IdRecipient = idRecipient
	updatedMsg.IdMessageRecipientType = recipientType

	updateSource := ""
	if isBot, err := mc.userRepo.IsBotUser(ctx, user.IdUser); err == nil && isBot {
		updateSource = "bot"
	}

	switch recipientType {
	case model.TeammateRecipientType:
		mc.notifier.Send <- &notify.Notice{
			IdUser:  idRecipient,
			Subject: notify.SubjectMessage,
			Action:  notify.ActionUpdate,
			Payload: updatedMsg,
			Source:  updateSource,
		}
	case model.TeamRecipientType:
		users, err := mc.teamRepo.LoadTeamsMembers(ctx, []int64{idRecipient})
		if err == nil {
			for _, u := range users {
				mc.notifier.Send <- &notify.Notice{
					IdUser:  u.IdUser,
					Subject: notify.SubjectMessage,
					Action:  notify.ActionUpdate,
					Payload: updatedMsg,
					Source:  updateSource,
				}
			}
		}
	case model.ProjectRecipientType:
		users, err := mc.projectRepo.LoadProjectsMembers(ctx, []int64{idRecipient})
		if err == nil {
			for _, u := range users {
				mc.notifier.Send <- &notify.Notice{
					IdUser:  u.IdUser,
					Subject: notify.SubjectMessage,
					Action:  notify.ActionUpdate,
					Payload: updatedMsg,
					Source:  updateSource,
				}
			}
		}
	case model.IssueRecipientType:
		project, err := mc.projectRepo.LoadProjectByIssue(ctx, idRecipient)
		if err == nil {
			users, err := mc.projectRepo.LoadProjectsMembers(ctx, []int64{project.IdProject})
			if err == nil {
				for _, u := range users {
					mc.notifier.Send <- &notify.Notice{
						IdUser:  u.IdUser,
						Subject: notify.SubjectMessage,
						Action:  notify.ActionUpdate,
						Payload: updatedMsg,
						Source:  updateSource,
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, updatedMsg)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// handleAgentRunHook reacts to a user comment on an issue with an active agent
// run in a passive phase (awaiting_approval or awaiting_input): it enqueues a
// new agent_task for the same stage as the last completed task and returns
// the run to queued so the scheduler picks it up. Bot-authored plan/
// clarification messages do NOT come through here — those go through
// submit_plan / request_clarification, which own their own phase transitions.
func (mc *MessageController) handleAgentRunHook(ctx context.Context, author *model.User, msg *model.Message, idIssue int64) {
	if author.IsBot {
		return
	}
	if msg.MessageKind != constants.MessageKindComment {
		return
	}
	activeRun, err := mc.agentRunRepo.LoadActiveByIssue(ctx, idIssue)
	if err != nil || activeRun == nil {
		return
	}
	if !constants.PassivePhases[activeRun.Phase] {
		return
	}

	tasks, err := mc.agentTaskRepo.LoadByRun(ctx, activeRun.IdRun)
	if err != nil {
		return
	}
	var lastCompleted *model.AgentTask
	for _, t := range tasks {
		if t.Status == constants.TaskStatusCompleted {
			if lastCompleted == nil || t.CreatedAt.After(lastCompleted.CreatedAt) {
				lastCompleted = t
			}
		}
	}
	if lastCompleted == nil {
		return
	}

	attemptNo := agent.ResolveNextAttemptNo(tasks, lastCompleted.Stage)
	if _, err := mc.agentTaskRepo.Insert(ctx, activeRun.IdRun, activeRun.IdUserBot, lastCompleted.Stage, attemptNo); err != nil {
		return
	}
	idUser := author.IdUser
	updated, transErr := mc.agentRunRepo.TransitionPhase(
		ctx, activeRun.IdRun, activeRun.Phase, constants.PhaseQueued,
		constants.ActorTypeUser, &idUser, "comment_triggered_revision",
	)
	if transErr == nil {
		agent.BroadcastRunUpdate(ctx, mc.notifier, mc.projectRepo, mc.agentRunRepo, mc.agentTaskRepo, updated)
	}
}
