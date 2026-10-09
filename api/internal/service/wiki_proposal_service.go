package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikimerge"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikitext"
)

type WikiProposalService struct {
	wiki          *WikiService
	wikiAgent     *WikiAgentService
	proposalRepo  *repository.WikiProposalRepository
	issueRepo     *repository.IssueRepository
	messageRepo   *repository.MessageRepository
	userRepo      *repository.UserRepository
	notifications *NotificationService
}

type WikiProposalDetail struct {
	Proposal  *model.WikiProposal `json:"proposal"`
	Diff      string              `json:"diff"`
	Conflicts int                 `json:"conflicts"`
}

type WikiProposalAcceptResult struct {
	Proposal   *model.WikiProposal `json:"proposal"`
	Page       *model.WikiPage     `json:"page"`
	MergedFrom *int                `json:"mergedFrom"`
	Conflict   *WikiConflictInfo   `json:"-"`
}

func NewWikiProposalService(
	wiki *WikiService,
	wikiAgent *WikiAgentService,
	proposalRepo *repository.WikiProposalRepository,
	issueRepo *repository.IssueRepository,
	messageRepo *repository.MessageRepository,
	userRepo *repository.UserRepository,
	notifications *NotificationService,
) *WikiProposalService {
	return &WikiProposalService{
		wiki:          wiki,
		wikiAgent:     wikiAgent,
		proposalRepo:  proposalRepo,
		issueRepo:     issueRepo,
		messageRepo:   messageRepo,
		userRepo:      userRepo,
		notifications: notifications,
	}
}

func (s *WikiProposalService) Suggest(ctx context.Context, user model.User, idProject, idRun int64, req model.WikiProposalSuggestReq) (*model.WikiProposalAgentView, error) {
	if !user.IsAgent {
		return nil, errs.ErrForbidden.WithMessage("only an agent run can suggest wiki changes; edit the wiki directly instead")
	}
	if !constants.IsValidWikiProposalKind(req.Kind) {
		return nil, errs.ErrBadRequest.WithMessage("kind must be create, update, move or delete")
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, errs.ErrBadRequest.WithMessage("reason is required: say what in the code makes the wiki change necessary")
	}
	run, err := s.implementationRun(ctx, user, idRun, idProject)
	if err != nil {
		return nil, err
	}
	instance, project, err := s.wiki.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	draft := &model.WikiProposalDraft{IdRun: run.IdRun, IdIssue: run.IdIssue, IdProject: idProject, Kind: req.Kind, Reason: reason}
	if req.Kind == constants.WikiProposalCreate {
		err = s.draftCreate(ctx, draft, instance, project, req)
	} else {
		err = s.draftChange(ctx, draft, instance, project, req)
	}
	if err != nil {
		return nil, err
	}
	idProposal, isInserted, err := s.proposalRepo.Upsert(ctx, draft)
	if errors.Is(err, repository.ErrWikiProposalDecided) {
		return nil, errs.ErrWikiProposalDecided
	}
	if err != nil {
		return nil, err
	}
	proposal, err := s.proposalRepo.LoadOne(ctx, idProposal, false)
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, errs.ErrNotFound
	}
	s.broadcast(ctx, proposal.IdProject, proposal.IdIssue)
	view := agentProposalView(proposal)
	view.Updated = !isInserted
	return &view, nil
}

func (s *WikiProposalService) LoadForAgentRun(ctx context.Context, user model.User, idProject, idRun int64) ([]model.WikiProposalAgentView, error) {
	run, err := s.wikiAgent.runRepo.LoadById(ctx, idRun)
	if errors.Is(err, repository.ErrRunNotFound) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if run.IdProject != idProject || (user.IsAgent && run.IdUserAgent != user.IdUser) {
		return nil, errs.ErrNotFound
	}
	if !s.wiki.acl.CanReadProject(ctx, user.IdUser, run.IdProject) {
		return nil, errs.ErrForbidden
	}
	proposals, err := s.proposalRepo.Load(ctx, model.WikiProposalFilter{IdRun: &idRun})
	if err != nil {
		return nil, err
	}
	views := make([]model.WikiProposalAgentView, len(proposals))
	for i, proposal := range proposals {
		views[i] = agentProposalView(proposal)
	}
	return views, nil
}

func (s *WikiProposalService) LoadForPrompt(ctx context.Context, idRun int64) ([]model.WikiProposalAgentView, error) {
	proposals, err := s.proposalRepo.Load(ctx, model.WikiProposalFilter{IdRun: &idRun, Decisions: constants.WikiProposalRevisable})
	if err != nil {
		return nil, err
	}
	views := make([]model.WikiProposalAgentView, len(proposals))
	for i, proposal := range proposals {
		views[i] = agentProposalView(proposal)
	}
	return views, nil
}

func (s *WikiProposalService) LoadByIssue(ctx context.Context, user model.User, idIssue int64) ([]*model.WikiProposal, error) {
	project, err := s.wiki.projectOfIssue(ctx, idIssue)
	if err != nil {
		return nil, err
	}
	if !s.wiki.acl.CanReadProject(ctx, user.IdUser, project.IdProject) {
		return nil, errs.ErrForbidden
	}
	return s.proposalRepo.Load(ctx, model.WikiProposalFilter{IdIssue: &idIssue})
}

func (s *WikiProposalService) LoadOpen(ctx context.Context, user model.User, idProject int64) ([]*model.WikiProposal, error) {
	if _, _, err := s.wiki.projectSpaces(ctx, user, idProject); err != nil {
		return nil, err
	}
	if !s.wiki.acl.CanReadProject(ctx, user.IdUser, idProject) {
		return nil, errs.ErrForbidden
	}
	return s.proposalRepo.Load(ctx, model.WikiProposalFilter{IdProject: &idProject, Decisions: constants.WikiProposalUndecided, OnlyLive: true})
}

func (s *WikiProposalService) LoadDetail(ctx context.Context, user model.User, idProposal int64) (*WikiProposalDetail, error) {
	proposal, err := s.proposalRepo.LoadOne(ctx, idProposal, false)
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, errs.ErrNotFound
	}
	if !s.wiki.acl.CanReadProject(ctx, user.IdUser, proposal.IdProject) {
		return nil, errs.ErrForbidden
	}
	detail := &WikiProposalDetail{Proposal: proposal}
	if proposal.Body == nil {
		return detail, nil
	}
	base := ""
	if proposal.Kind == constants.WikiProposalUpdate && proposal.IdPage != nil {
		version, err := s.wiki.versionRepo.LoadOne(ctx, *proposal.IdPage, *proposal.BaseVersion)
		if err != nil {
			return nil, err
		}
		if version != nil {
			base = version.Body
		}
		if page, err := s.wiki.pageRepo.LoadPage(ctx, *proposal.IdPage); err != nil {
			return nil, err
		} else if page != nil && version != nil && page.VersionNo > version.VersionNo {
			detail.Conflicts = wikimerge.Merge(version.Body, *proposal.Body, page.Body).Conflicts
		}
	}
	detail.Diff, err = wikimerge.UnifiedDiff(withTrailingNewline(base), withTrailingNewline(*proposal.Body), proposal.Title, proposal.Title)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *WikiProposalService) Accept(ctx context.Context, user model.User, idProposal int64, req model.WikiProposalAcceptReq) (*WikiProposalAcceptResult, error) {
	if user.IsAgent {
		return nil, errs.ErrForbidden.WithMessage("agents cannot accept wiki proposals")
	}
	var result *WikiProposalAcceptResult
	err := extctx.RunInTx(ctx, s.wiki.pool, func(ctx context.Context) error {
		proposal, err := s.lockDecidable(ctx, user, idProposal)
		if err != nil {
			return err
		}
		switch proposal.State {
		case constants.WikiProposalReady, constants.WikiProposalStateNeedsResolving:
			result, err = s.publish(ctx, user, proposal, req, constants.WikiProposalPublishing)
		default:
			result, err = s.approve(ctx, user, proposal, req)
		}
		if err != nil || result.Conflict != nil {
			return err
		}
		result.Proposal, err = s.proposalRepo.LoadOne(ctx, proposal.IdProposal, false)
		if err != nil {
			return err
		}
		s.broadcast(ctx, proposal.IdProject, proposal.IdIssue)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *WikiProposalService) Reject(ctx context.Context, user model.User, idProposal int64, req model.WikiProposalRejectReq) (*model.WikiProposal, error) {
	if user.IsAgent {
		return nil, errs.ErrForbidden.WithMessage("agents cannot reject wiki proposals")
	}
	var rejected *model.WikiProposal
	err := extctx.RunInTx(ctx, s.wiki.pool, func(ctx context.Context) error {
		proposal, err := s.lockDecidable(ctx, user, idProposal)
		if err != nil {
			return err
		}
		reason := strings.TrimSpace(req.Reason)
		var note *string
		if reason != "" {
			note = &reason
		}
		if err := s.proposalRepo.Decide(ctx, proposal.IdProposal, constants.WikiProposalRejected, constants.WikiProposalUndecided, user.IdUser, note, nil, nil); err != nil {
			return s.decideErr(err)
		}
		message, err := s.messageRepo.InsertIssueMessage(ctx, rejectionComment(proposal, reason), &user, proposal.IdIssue, nil)
		if err != nil {
			return err
		}
		rejected, err = s.proposalRepo.LoadOne(ctx, proposal.IdProposal, false)
		if err != nil {
			return err
		}
		s.broadcastMessage(ctx, proposal.IdProject, user.IdUser, message)
		s.broadcast(ctx, proposal.IdProject, proposal.IdIssue)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rejected, nil
}

func (s *WikiProposalService) RunFinished(ctx context.Context, run *model.AgentRun) {
	idRun := run.IdRun
	proposals, err := s.proposalRepo.Load(ctx, model.WikiProposalFilter{IdRun: &idRun, Decisions: constants.WikiProposalUndecided})
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Int64("idRun", idRun).Msg("wiki proposals: loading for a finished run")
		return
	}
	if len(proposals) == 0 {
		return
	}
	if run.Phase == constants.PhaseDone {
		var ready, unresolved []*model.WikiProposal
		for _, proposal := range proposals {
			switch proposal.Decision {
			case constants.WikiProposalOpen:
				ready = append(ready, proposal)
			case constants.WikiProposalApproved:
				if !s.publishApproved(ctx, proposal) {
					unresolved = append(unresolved, proposal)
				}
			}
		}
		s.notifyReady(ctx, run, ready)
		s.notifyUnresolved(ctx, run, unresolved)
	}
	s.broadcast(ctx, run.IdProject, run.IdIssue)
}

func (s *WikiProposalService) implementationRun(ctx context.Context, user model.User, idRun, idProject int64) (*model.AgentRun, error) {
	if idRun == 0 {
		return nil, errs.ErrBadRequest.WithMessage("wiki changes can only be suggested from inside an agent run")
	}
	ref := s.wikiAgent.runRef(ctx, user, idRun, idProject)
	if ref == nil {
		return nil, errs.ErrForbidden.WithMessage("wiki changes can only be suggested by the agent of an unfinished run in this project")
	}
	if ref.stage != constants.StageImplementation {
		return nil, errs.ErrConflict.WithMessage("wiki changes can only be suggested in the implementation stage")
	}
	run, err := s.wikiAgent.runRepo.LoadById(ctx, idRun)
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (s *WikiProposalService) draftCreate(ctx context.Context, draft *model.WikiProposalDraft, instance, project *model.WikiSpace, req model.WikiProposalSuggestReq) error {
	title := strings.TrimSpace(req.Title)
	slug := wikitext.Slugify(title)
	if slug == "" {
		return errs.ErrWikiInvalidTitle
	}
	target := req.Slug
	if target == "" {
		target = title
	}
	link, ok := wikitext.ParseTarget(target)
	if !ok {
		return errs.ErrWikiInvalidTitle
	}
	if link.Slug != slug {
		return errs.ErrBadRequest.WithMessage(fmt.Sprintf(
			"a new page gets its slug from the title; title %q gives %q, not %q", title, slug, link.Slug))
	}
	if req.Body == nil {
		return errs.ErrBadRequest.WithMessage("body is required to propose a new page")
	}
	kind := constants.WikiSpaceProject
	if link.Shared || req.Space == constants.WikiSpaceInstance {
		kind = constants.WikiSpaceInstance
	}
	space := s.wiki.spaceByKind(instance, project, kind)
	existing, err := s.wiki.pageRepo.LoadLivePageBySlug(ctx, space.IdSpace, slug)
	if err != nil {
		return err
	}
	if existing != nil {
		return errs.ErrWikiSlugTaken.WithMessage(fmt.Sprintf(
			"page %q already exists; propose kind=update with the base_version you read", agentSlugIn(kind, slug)))
	}
	idParent, err := s.wikiAgent.parentId(ctx, space, req.Parent)
	if err != nil {
		return err
	}
	draft.IdSpace, draft.Slug, draft.Title, draft.Body, draft.IdParent = space.IdSpace, slug, title, req.Body, idParent
	if req.Summary != nil {
		draft.Summary = strings.TrimSpace(*req.Summary)
	}
	if idParent != nil {
		draft.ParentSlug = parentSlugOf(req.Parent)
	}
	return nil
}

func (s *WikiProposalService) draftChange(ctx context.Context, draft *model.WikiProposalDraft, instance, project *model.WikiSpace, req model.WikiProposalSuggestReq) error {
	if req.Slug == "" {
		return errs.ErrBadRequest.WithMessage("slug of the page to change is required")
	}
	link, ok := wikitext.ParseTarget(req.Slug)
	if !ok {
		return errs.ErrNotFound
	}
	space, page, err := s.wikiAgent.findPage(ctx, instance, project, link)
	if err != nil {
		return err
	}
	if page == nil {
		return errs.ErrNotFound.WithMessage(fmt.Sprintf("page %q not found", req.Slug))
	}
	draft.IdSpace, draft.IdPage, draft.Slug, draft.Title, draft.Summary = space.IdSpace, &page.IdPage, page.Slug, page.Title, page.Summary
	baseVersion := page.VersionNo
	draft.BaseVersion = &baseVersion
	switch req.Kind {
	case constants.WikiProposalUpdate:
		return s.draftUpdate(draft, page, req)
	case constants.WikiProposalMove:
		return s.draftMove(ctx, draft, space, page, req)
	default:
		return nil
	}
}

func (s *WikiProposalService) draftUpdate(draft *model.WikiProposalDraft, page *model.WikiPage, req model.WikiProposalSuggestReq) error {
	if req.Body == nil {
		return errs.ErrBadRequest.WithMessage("body is required: pass the whole new content of the page")
	}
	if req.BaseVersion <= 0 || req.BaseVersion > page.VersionNo {
		return errs.ErrBadRequest.WithMessage(fmt.Sprintf(
			"base_version is required: the version of %q you based the change on (the current one is %d)", page.Slug, page.VersionNo))
	}
	if title := strings.TrimSpace(req.Title); title != "" {
		if wikitext.Slugify(title) == "" {
			return errs.ErrWikiInvalidTitle
		}
		draft.Title = title
	}
	if req.Summary != nil {
		draft.Summary = strings.TrimSpace(*req.Summary)
	}
	baseVersion := req.BaseVersion
	draft.Body, draft.BaseVersion = req.Body, &baseVersion
	return nil
}

func (s *WikiProposalService) draftMove(ctx context.Context, draft *model.WikiProposalDraft, space *model.WikiSpace, page *model.WikiPage, req model.WikiProposalSuggestReq) error {
	if req.Parent == nil {
		return errs.ErrBadRequest.WithMessage("parent is required to move a page; pass an empty string for the top level")
	}
	idParent, err := s.wikiAgent.parentId(ctx, space, req.Parent)
	if err != nil {
		return err
	}
	if err := s.wiki.validateParent(ctx, space.IdSpace, idParent, page.IdPage); err != nil {
		return err
	}
	if sameParent(idParent, page.IdParent) {
		return errs.ErrBadRequest.WithMessage(fmt.Sprintf("page %q is already there", req.Slug))
	}
	draft.IdParent = idParent
	if idParent != nil {
		draft.ParentSlug = parentSlugOf(req.Parent)
	}
	return nil
}

func (s *WikiProposalService) lockDecidable(ctx context.Context, user model.User, idProposal int64) (*model.WikiProposal, error) {
	proposal, err := s.proposalRepo.LoadOne(ctx, idProposal, true)
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, errs.ErrNotFound
	}
	if !s.wiki.acl.CanReadProject(ctx, user.IdUser, proposal.IdProject) {
		return nil, errs.ErrForbidden
	}
	switch proposal.State {
	case constants.WikiProposalStateAccepted, constants.WikiProposalStateRejected:
		return nil, errs.ErrWikiProposalDecided
	case constants.WikiProposalDiscarded:
		return nil, errs.ErrWikiProposalNotReady.WithMessage("the run of this proposal ended without a merged pull request")
	}
	space, err := s.wiki.spaceRepo.LoadSpace(ctx, proposal.IdSpace)
	if err != nil {
		return nil, err
	}
	if space == nil {
		return nil, errs.ErrNotFound
	}
	if !s.wiki.spaceAccess(ctx, user, space).canEdit {
		return nil, errs.ErrForbidden
	}
	return proposal, nil
}

func (s *WikiProposalService) approve(ctx context.Context, user model.User, proposal *model.WikiProposal, req model.WikiProposalAcceptReq) (*WikiProposalAcceptResult, error) {
	title, summary, body := acceptedContent(proposal, req)
	if wikitext.Slugify(title) == "" {
		return nil, errs.ErrWikiInvalidTitle
	}
	approval := model.WikiProposalApproval{IdUser: user.IdUser, Title: title, Summary: summary}
	if note := strings.TrimSpace(req.Note); note != "" {
		approval.Note = &note
	}
	if req.AgentAccess != "" {
		agentAccess := req.AgentAccess
		approval.AgentAccess = &agentAccess
	}
	if (proposal.Kind == constants.WikiProposalCreate || proposal.Kind == constants.WikiProposalMove) &&
		proposal.ParentSlug != nil && proposal.IdParent == nil {
		return nil, errs.ErrWikiProposalParentGone
	}
	switch proposal.Kind {
	case constants.WikiProposalCreate:
		approval.Body = &body
	case constants.WikiProposalUpdate:
		page, err := s.livePage(ctx, proposal)
		if err != nil {
			return nil, err
		}
		baseVersion := *proposal.BaseVersion
		if req.BaseVersion > 0 {
			baseVersion = req.BaseVersion
		}
		conflict, err := s.conflictWith(ctx, page, baseVersion, title, summary, body, req.AgentAccess)
		if err != nil || conflict != nil {
			return &WikiProposalAcceptResult{Conflict: conflict}, err
		}
		approval.Body, approval.BaseVersion = &body, &baseVersion
	default:
		if _, err := s.livePage(ctx, proposal); err != nil {
			return nil, err
		}
	}
	if err := s.proposalRepo.Approve(ctx, proposal.IdProposal, approval); err != nil {
		return nil, s.decideErr(err)
	}
	return &WikiProposalAcceptResult{}, nil
}

func (s *WikiProposalService) conflictWith(ctx context.Context, page *model.WikiPage, baseVersion int, title, summary, body string, agentAccess constants.WikiAgentAccess) (*WikiConflictInfo, error) {
	if baseVersion > page.VersionNo {
		return nil, errs.ErrBadRequest
	}
	if baseVersion == page.VersionNo {
		return nil, nil
	}
	base, err := s.wiki.versionRepo.LoadOne(ctx, page.IdPage, baseVersion)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return nil, errs.ErrBadRequest
	}
	merged := wikimerge.Merge(base.Body, body, page.Body)
	if !merged.HasConflicts() {
		return nil, nil
	}
	fields := mergeWikiFields(base, title, summary, agentAccess, page)
	return &WikiConflictInfo{
		Current:     page,
		Title:       fields.title,
		Summary:     fields.summary,
		AgentAccess: fields.agentAccess,
		Merge:       merged,
	}, nil
}

func (s *WikiProposalService) publish(ctx context.Context, user model.User, proposal *model.WikiProposal, req model.WikiProposalAcceptReq, from []constants.WikiProposalDecision) (*WikiProposalAcceptResult, error) {
	note := proposalNote(proposal, req.Note)
	var result *WikiProposalAcceptResult
	var err error
	switch proposal.Kind {
	case constants.WikiProposalCreate:
		result, err = s.acceptCreate(ctx, user, proposal, req, note)
	case constants.WikiProposalUpdate:
		result, err = s.acceptUpdate(ctx, user, proposal, req, note)
	case constants.WikiProposalMove:
		result, err = s.acceptMove(ctx, user, proposal)
	case constants.WikiProposalDelete:
		result, err = s.acceptDelete(ctx, user, proposal)
	default:
		return nil, errs.ErrBadRequest
	}
	if err != nil || result.Conflict != nil {
		return result, err
	}
	var idPage *int64
	var resultVersion *int
	if result.Page != nil {
		idPage, resultVersion = &result.Page.IdPage, &result.Page.VersionNo
	}
	if err := s.proposalRepo.Decide(ctx, proposal.IdProposal, constants.WikiProposalAccepted, from, user.IdUser, nil, idPage, resultVersion); err != nil {
		return nil, s.decideErr(err)
	}
	return result, nil
}

func (s *WikiProposalService) publishApproved(ctx context.Context, proposal *model.WikiProposal) bool {
	log := extctx.GetLogger(ctx).With().Int64("idProposal", proposal.IdProposal).Logger()
	isSettled := false
	if proposal.DecidedBy != nil {
		approver, err := s.userRepo.LoadUser(ctx, *proposal.DecidedBy)
		if err == nil {
			err = extctx.RunInTx(ctx, s.wiki.pool, func(ctx context.Context) error {
				locked, err := s.proposalRepo.LoadOne(ctx, proposal.IdProposal, true)
				if err != nil {
					return err
				}
				if locked == nil || locked.Decision != constants.WikiProposalApproved {
					isSettled = true
					return nil
				}
				result, err := s.publish(ctx, *approver, locked, approvedRequest(locked), []constants.WikiProposalDecision{constants.WikiProposalApproved})
				if err != nil {
					return err
				}
				if result.Conflict != nil {
					return errs.ErrConflict
				}
				isSettled = true
				return nil
			})
		}
		if err != nil {
			log.Warn().Err(err).Msg("wiki proposals: publishing an approved proposal after the merge")
		}
	}
	if isSettled {
		return true
	}
	if err := s.proposalRepo.MarkNeedsResolving(ctx, proposal.IdProposal); err != nil {
		log.Error().Err(err).Msg("wiki proposals: marking an approved proposal as needing resolution")
	}
	return false
}

func (s *WikiProposalService) acceptCreate(ctx context.Context, user model.User, proposal *model.WikiProposal, req model.WikiProposalAcceptReq, note string) (*WikiProposalAcceptResult, error) {
	if proposal.ParentSlug != nil && proposal.IdParent == nil {
		return nil, errs.ErrWikiProposalParentGone
	}
	title, summary, body := acceptedContent(proposal, req)
	created, err := s.wiki.Create(ctx, user, proposal.IdProject, model.CreateWikiPageReq{
		Space:       proposal.SpaceKind,
		IdParent:    proposal.IdParent,
		Title:       title,
		Summary:     summary,
		Body:        body,
		AgentAccess: req.AgentAccess,
		Note:        note,
	})
	if err != nil {
		return nil, err
	}
	return &WikiProposalAcceptResult{Page: created}, nil
}

func (s *WikiProposalService) acceptUpdate(ctx context.Context, user model.User, proposal *model.WikiProposal, req model.WikiProposalAcceptReq, note string) (*WikiProposalAcceptResult, error) {
	page, err := s.livePage(ctx, proposal)
	if err != nil {
		return nil, err
	}
	baseVersion := *proposal.BaseVersion
	if req.BaseVersion > 0 {
		baseVersion = req.BaseVersion
	}
	title, summary, body := acceptedContent(proposal, req)
	agentAccess := page.AgentAccess
	if req.AgentAccess != "" {
		agentAccess = req.AgentAccess
	}
	saved, err := s.wiki.Save(ctx, user, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: baseVersion,
		Title:       title,
		Summary:     summary,
		Body:        body,
		AgentAccess: agentAccess,
		Note:        note,
	})
	if err != nil {
		return nil, err
	}
	return &WikiProposalAcceptResult{Page: saved.Page, MergedFrom: saved.MergedFrom, Conflict: saved.Conflict}, nil
}

func (s *WikiProposalService) acceptMove(ctx context.Context, user model.User, proposal *model.WikiProposal) (*WikiProposalAcceptResult, error) {
	page, err := s.livePage(ctx, proposal)
	if err != nil {
		return nil, err
	}
	if proposal.ParentSlug != nil && proposal.IdParent == nil {
		return nil, errs.ErrWikiProposalParentGone
	}
	if err := s.wiki.Move(ctx, user, page.IdPage, model.MoveWikiPageReq{IdParent: proposal.IdParent}); err != nil {
		return nil, err
	}
	return &WikiProposalAcceptResult{Page: page}, nil
}

func (s *WikiProposalService) acceptDelete(ctx context.Context, user model.User, proposal *model.WikiProposal) (*WikiProposalAcceptResult, error) {
	page, err := s.livePage(ctx, proposal)
	if err != nil {
		return nil, err
	}
	if err := s.wiki.Trash(ctx, user, page.IdPage); err != nil {
		return nil, err
	}
	return &WikiProposalAcceptResult{}, nil
}

func (s *WikiProposalService) livePage(ctx context.Context, proposal *model.WikiProposal) (*model.WikiPage, error) {
	if proposal.IdPage == nil || !proposal.IsPageLive {
		return nil, errs.ErrWikiProposalPageGone
	}
	page, err := s.wiki.pageRepo.LoadPage(ctx, *proposal.IdPage)
	if err != nil {
		return nil, err
	}
	if page == nil || page.DeletedAt != nil {
		return nil, errs.ErrWikiProposalPageGone
	}
	return page, nil
}

func (s *WikiProposalService) decideErr(err error) error {
	if errors.Is(err, repository.ErrWikiProposalDecided) {
		return errs.ErrWikiProposalDecided
	}
	return err
}

func (s *WikiProposalService) notifyReady(ctx context.Context, run *model.AgentRun, proposals []*model.WikiProposal) {
	if len(proposals) == 0 {
		return
	}
	s.notify(ctx, run, constants.NotificationTypeWikiProposalReady, func(issue *model.Issue, _ map[int64]bool) map[int64][]*model.WikiProposal {
		recipients := map[int64][]*model.WikiProposal{issue.CreateBy: proposals}
		if issue.AssignedTo != nil {
			recipients[*issue.AssignedTo] = proposals
		}
		return recipients
	})
}

func (s *WikiProposalService) notifyUnresolved(ctx context.Context, run *model.AgentRun, proposals []*model.WikiProposal) {
	if len(proposals) == 0 {
		return
	}
	s.notify(ctx, run, constants.NotificationTypeWikiProposalConflict, func(issue *model.Issue, isPerson map[int64]bool) map[int64][]*model.WikiProposal {
		recipients := map[int64][]*model.WikiProposal{}
		for _, proposal := range proposals {
			if proposal.DecidedBy != nil && isPerson[*proposal.DecidedBy] {
				recipients[*proposal.DecidedBy] = append(recipients[*proposal.DecidedBy], proposal)
				continue
			}
			recipients[issue.CreateBy] = append(recipients[issue.CreateBy], proposal)
			if issue.AssignedTo != nil && *issue.AssignedTo != issue.CreateBy {
				recipients[*issue.AssignedTo] = append(recipients[*issue.AssignedTo], proposal)
			}
		}
		return recipients
	})
}

func (s *WikiProposalService) notify(ctx context.Context, run *model.AgentRun, notificationType string, recipientsOf func(issue *model.Issue, isPerson map[int64]bool) map[int64][]*model.WikiProposal) {
	log := extctx.GetLogger(ctx).With().Int64("idRun", run.IdRun).Str("type", notificationType).Logger()
	idIssue := run.IdIssue
	issue, err := s.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &idIssue})
	if err != nil || issue == nil {
		log.Error().Err(err).Msg("wiki proposals: loading the issue to notify")
		return
	}
	project, err := s.wiki.projectRepo.LoadProject(ctx, run.IdProject)
	if err != nil {
		log.Error().Err(err).Msg("wiki proposals: loading the project to notify")
		return
	}
	members, err := s.wiki.projectRepo.LoadProjectsMembers(ctx, []int64{run.IdProject})
	if err != nil {
		log.Error().Err(err).Msg("wiki proposals: loading members to notify")
		return
	}
	agent := &model.User{}
	isPerson := map[int64]bool{}
	for _, member := range members {
		if member.IdUser == run.IdUserAgent {
			agent = member
		}
		if !member.IsAgent {
			isPerson[member.IdUser] = true
		}
	}
	idProject, idIssuePublic := project.IdProject, issue.IdIssuePublic
	for idUser, proposals := range recipientsOf(issue, isPerson) {
		if !isPerson[idUser] {
			continue
		}
		err := s.notifications.Notify(ctx, &model.CreateNotificationReq{
			IdUser:        idUser,
			Type:          notificationType,
			IdProject:     &idProject,
			ProjectName:   project.Name,
			ProjectColor:  project.Color,
			ActorName:     agent.Name,
			ActorAvatarBg: agent.ColorAvatarBg,
			RefType:       constants.NotificationRefTypeIssue,
			RefId:         fmt.Sprint(issue.IdIssue),
			RefTitle:      issue.Title,
			RefPublicId:   &idIssuePublic,
			Source:        constants.NotificationSourceAgent,
			Body: model.NotificationBodyWikiProposal{
				Count:      len(proposals),
				IdProposal: proposals[0].IdProposal,
				Title:      proposals[0].Title,
			},
		})
		if err != nil {
			log.Error().Err(err).Int64("idUser", idUser).Msg("wiki proposals: notifying")
		}
	}
}

func (s *WikiProposalService) broadcast(ctx context.Context, idProject, idIssue int64) {
	if s.wiki.notifier == nil {
		return
	}
	extctx.AfterCommit(ctx, func(ctx context.Context) {
		idsUser, err := s.memberIds(ctx, idProject, 0)
		if err != nil || len(idsUser) == 0 {
			return
		}
		s.wiki.notifier.Send <- &notify.Notice{
			IdsUser: idsUser,
			Subject: notify.SubjectWikiProposal,
			Action:  notify.ActionUpdate,
			Payload: model.WikiProposalNotice{IdProject: idProject, IdIssue: idIssue},
		}
	})
}

func (s *WikiProposalService) broadcastMessage(ctx context.Context, idProject, idAuthor int64, message *model.Message) {
	if s.wiki.notifier == nil {
		return
	}
	extctx.AfterCommit(ctx, func(ctx context.Context) {
		idsUser, err := s.memberIds(ctx, idProject, idAuthor)
		if err != nil || len(idsUser) == 0 {
			return
		}
		s.wiki.notifier.Send <- &notify.Notice{
			IdsUser: idsUser,
			Subject: notify.SubjectMessage,
			Action:  notify.ActionCreate,
			Payload: message,
		}
	})
}

func (s *WikiProposalService) memberIds(ctx context.Context, idProject, idExcluded int64) ([]int64, error) {
	members, err := s.wiki.projectRepo.LoadProjectsMembers(ctx, []int64{idProject})
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Int64("idProject", idProject).Msg("wiki proposals: loading members for a notice")
		return nil, err
	}
	idsUser := make([]int64, 0, len(members))
	for _, member := range members {
		if member.IdUser != idExcluded {
			idsUser = append(idsUser, member.IdUser)
		}
	}
	return idsUser, nil
}

func agentProposalView(proposal *model.WikiProposal) model.WikiProposalAgentView {
	view := model.WikiProposalAgentView{
		Id:          proposal.IdProposal,
		Kind:        proposal.Kind,
		Slug:        agentSlugIn(proposal.SpaceKind, proposal.Slug),
		Title:       proposal.Title,
		Reason:      proposal.Reason,
		BaseVersion: proposal.BaseVersion,
		State:       proposal.State,
	}
	if proposal.ParentSlug != nil {
		parent := agentSlugIn(proposal.SpaceKind, *proposal.ParentSlug)
		view.Parent = &parent
	}
	return view
}

func acceptedContent(proposal *model.WikiProposal, req model.WikiProposalAcceptReq) (string, string, string) {
	title, summary, body := proposal.Title, proposal.Summary, ""
	if proposal.Body != nil {
		body = *proposal.Body
	}
	if req.Title != nil {
		title = *req.Title
	}
	if req.Summary != nil {
		summary = *req.Summary
	}
	if req.Body != nil {
		body = *req.Body
	}
	return title, summary, body
}

func approvedRequest(proposal *model.WikiProposal) model.WikiProposalAcceptReq {
	req := model.WikiProposalAcceptReq{Title: &proposal.Title, Summary: &proposal.Summary, Body: proposal.Body}
	if proposal.BaseVersion != nil {
		req.BaseVersion = *proposal.BaseVersion
	}
	if proposal.AgentAccess != nil {
		req.AgentAccess = *proposal.AgentAccess
	}
	if proposal.DecisionNote != nil {
		req.Note = *proposal.DecisionNote
	}
	return req
}

func proposalNote(proposal *model.WikiProposal, note string) string {
	text := fmt.Sprintf("Agent proposal from #%d (run #%d)", proposal.IdIssuePublic, proposal.IdRun)
	if note = strings.TrimSpace(note); note != "" {
		text += ": " + note
	}
	return text
}

func rejectionComment(proposal *model.WikiProposal, reason string) string {
	text := fmt.Sprintf("Rejected the wiki proposal to %s **%s** (`%s`).", proposal.Kind, proposal.Title, agentSlugIn(proposal.SpaceKind, proposal.Slug))
	if reason != "" {
		text += "\n\nReason: " + reason
	}
	return text
}

func parentSlugOf(parent *string) *string {
	if parent == nil {
		return nil
	}
	link, ok := wikitext.ParseTarget(*parent)
	if !ok {
		return nil
	}
	return &link.Slug
}
