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
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikitext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReleaseService struct {
	pool                *pgxpool.Pool
	releaseRepo         *repository.ReleaseRepository
	tagRepo             *repository.TagRepository
	issueRepo           *repository.IssueRepository
	userRepo            *repository.UserRepository
	agentRunRepo        *repository.AgentRunRepository
	agentTaskRepo       *repository.AgentTaskRepository
	agentGwRepo         *repository.AgentGatewayRepository
	stagePlan           *AgentStagePlanService
	projectSkillService *ProjectSkillService
	agentTaskConfig     *AgentTaskConfigService
	acl                 *AclService
	onReleaseChanged    func(ctx context.Context, idProject, idRelease int64)
}

type ReleaseServiceDeps struct {
	Pool                *pgxpool.Pool
	ReleaseRepo         *repository.ReleaseRepository
	TagRepo             *repository.TagRepository
	IssueRepo           *repository.IssueRepository
	UserRepo            *repository.UserRepository
	AgentRunRepo        *repository.AgentRunRepository
	AgentTaskRepo       *repository.AgentTaskRepository
	AgentGwRepo         *repository.AgentGatewayRepository
	StagePlan           *AgentStagePlanService
	ProjectSkillService *ProjectSkillService
	AgentTaskConfig     *AgentTaskConfigService
	Acl                 *AclService
}

func NewReleaseService(deps ReleaseServiceDeps) *ReleaseService {
	return &ReleaseService{
		pool:                deps.Pool,
		releaseRepo:         deps.ReleaseRepo,
		tagRepo:             deps.TagRepo,
		issueRepo:           deps.IssueRepo,
		userRepo:            deps.UserRepo,
		agentRunRepo:        deps.AgentRunRepo,
		agentTaskRepo:       deps.AgentTaskRepo,
		agentGwRepo:         deps.AgentGwRepo,
		stagePlan:           deps.StagePlan,
		projectSkillService: deps.ProjectSkillService,
		agentTaskConfig:     deps.AgentTaskConfig,
		acl:                 deps.Acl,
	}
}

func (s *ReleaseService) OnReleaseChanged(broadcast func(ctx context.Context, idProject, idRelease int64)) {
	s.onReleaseChanged = broadcast
}

type ReleaseResult struct {
	Release         *model.Release
	RenamedTag      *model.Tag
	CreatedTag      *model.Tag
	DeletedTag      *model.Tag
	IdsIssueChanged []int64
	Run             *model.AgentRun
	CancelledRun    *model.AgentRun
	IdsTaskStopped  []int64
}

func (s *ReleaseService) LoadDraft(ctx context.Context, idProject, idTag int64) (*model.ReleaseDraft, error) {
	if _, err := s.tagRepo.LoadTag(ctx, idProject, idTag); err != nil {
		return nil, tagErr(err)
	}
	issues, err := s.releaseRepo.LoadTagIssues(ctx, idProject, idTag)
	if err != nil {
		return nil, err
	}
	return buildReleaseDraft(issues), nil
}

func buildReleaseDraft(issues []*model.ReleaseIssue) *model.ReleaseDraft {
	draft := &model.ReleaseDraft{Sections: []*model.ReleaseSection{}, OpenIssues: []*model.ReleaseIssue{}}
	for _, issue := range issues {
		if !issue.IsClosed {
			draft.OpenIssues = append(draft.OpenIssues, issue)
			continue
		}
		last := len(draft.Sections) - 1
		if last < 0 || !sameIssueType(draft.Sections[last].IdIssueType, issue.IdIssueType) {
			draft.Sections = append(draft.Sections, &model.ReleaseSection{IdIssueType: issue.IdIssueType, Name: issue.IssueTypeName})
			last++
		}
		draft.Sections[last].Issues = append(draft.Sections[last].Issues, issue)
	}
	return draft
}

func sameIssueType(left, right *int64) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func releasedIssueCount(release *model.Release, draft *model.ReleaseDraft) int {
	count := 0
	for _, section := range draft.Sections {
		count += len(section.Issues)
	}
	if release.OpenIssues == constants.ReleaseOpenIssuesKeep {
		count += len(draft.OpenIssues)
	}
	return count
}

func (s *ReleaseService) CreateDraft(ctx context.Context, user model.User, idProject int64, req model.CreateReleaseReq) (*model.Release, error) {
	name, err := releaseName(req.Name)
	if err != nil {
		return nil, err
	}
	var created *model.Release
	err = extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		tag, err := s.tagRepo.LoadTag(ctx, idProject, req.IdTag)
		if err != nil {
			return tagErr(err)
		}
		if err := s.checkNameFree(ctx, idProject, 0, tag, name); err != nil {
			return err
		}
		created, err = s.releaseRepo.InsertDraft(ctx, &model.Release{
			IdProject: idProject,
			IdTag:     &tag.IdTag,
			Name:      name,
			CreateBy:  &user.IdUser,
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *ReleaseService) UpdateDraft(ctx context.Context, idProject, idRelease int64, req model.UpdateReleaseReq) (*model.Release, error) {
	name, err := releaseName(req.Name)
	if err != nil {
		return nil, err
	}
	var updated *model.Release
	err = extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		release, err := s.lockDraft(ctx, idProject, idRelease)
		if err != nil {
			return err
		}
		if req.Notes != nil && *req.Notes != release.Notes {
			if err := s.checkNoActiveRun(ctx, release); err != nil {
				return err
			}
			release.Notes = *req.Notes
		}
		if !strings.EqualFold(name, release.Name) {
			tag, err := s.draftTag(ctx, release)
			if err != nil {
				return err
			}
			if err := s.checkNameFree(ctx, idProject, release.IdRelease, tag, name); err != nil {
				return err
			}
		}
		release.Name = name
		release.RecreateTag = req.RecreateTag
		release.OpenIssues = req.OpenIssues
		if err := s.releaseRepo.UpdateDraft(ctx, release); err != nil {
			return err
		}
		updated = release
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *ReleaseService) StartAgent(ctx context.Context, idProject, idRelease int64, req model.StartReleaseAgentReq) (*ReleaseResult, error) {
	result := &ReleaseResult{}
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		release, err := s.lockDraft(ctx, idProject, idRelease)
		if err != nil {
			return err
		}
		if err := s.checkNoActiveRun(ctx, release); err != nil {
			return err
		}
		result.Release = release
		return s.startNotesRun(ctx, release, req, result)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ReleaseService) Publish(ctx context.Context, user model.User, idProject, idRelease int64) (*ReleaseResult, error) {
	result := &ReleaseResult{}
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		release, err := s.lockDraft(ctx, idProject, idRelease)
		if err != nil {
			return err
		}
		if err := s.checkNoActiveRun(ctx, release); err != nil {
			return err
		}
		if strings.TrimSpace(release.Notes) == "" {
			return errs.ErrValidation.WithMessage("release notes need some text")
		}
		tag, err := s.draftTag(ctx, release)
		if err != nil {
			return err
		}
		issues, err := s.releaseRepo.LoadTagIssues(ctx, idProject, tag.IdTag)
		if err != nil {
			return err
		}
		draft := buildReleaseDraft(issues)
		if err := s.checkNameFree(ctx, idProject, release.IdRelease, tag, release.Name); err != nil {
			return err
		}
		if release.RecreateTag && strings.EqualFold(release.Name, tag.Name) {
			return errs.ErrReleaseNameIsTag
		}
		if err := s.applyTag(ctx, release, tag, draft, result); err != nil {
			return err
		}
		release.IssueCount = releasedIssueCount(release, draft)
		release.ReleasedBy = &user.IdUser
		if err := s.releaseRepo.MarkReleased(ctx, release); err != nil {
			return err
		}
		result.Release = release
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ReleaseService) applyTag(ctx context.Context, release *model.Release, tag *model.Tag, draft *model.ReleaseDraft, result *ReleaseResult) error {
	sourceName := tag.Name
	tag.Name = release.Name
	if err := s.tagRepo.UpdateTag(ctx, tag); err != nil {
		if errors.Is(err, errs.ErrTagNameExists) {
			return errs.ErrReleaseNameTaken
		}
		return err
	}
	result.RenamedTag = tag

	var idTagNext *int64
	if release.RecreateTag {
		created, err := s.tagRepo.InsertTag(ctx, &model.Tag{IdProject: release.IdProject, Name: sourceName, Color: tag.Color})
		if err != nil {
			return err
		}
		result.CreatedTag = created
		idTagNext = &created.IdTag
	}

	if release.OpenIssues != constants.ReleaseOpenIssuesMove || len(draft.OpenIssues) == 0 {
		return nil
	}
	idsOpen := make([]int64, len(draft.OpenIssues))
	for i, issue := range draft.OpenIssues {
		idsOpen[i] = issue.IdIssue
	}
	if idTagNext != nil {
		if err := s.issueRepo.AddIssueTags(ctx, idsOpen, []int64{*idTagNext}); err != nil {
			return err
		}
	}
	if err := s.issueRepo.RemoveIssueTags(ctx, idsOpen, []int64{tag.IdTag}); err != nil {
		return err
	}
	result.IdsIssueChanged = idsOpen
	return nil
}

func (s *ReleaseService) UpdateReleasedNotes(ctx context.Context, idProject, idRelease int64, notes string) (*model.Release, error) {
	if strings.TrimSpace(notes) == "" {
		return nil, errs.ErrValidation.WithMessage("release notes need some text")
	}
	var updated *model.Release
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		release, err := s.releaseRepo.LockRelease(ctx, idProject, idRelease)
		if err != nil {
			return err
		}
		if release.Status != constants.ReleaseStatusReleased {
			return errs.ErrValidation.WithMessage("a draft saves its notes with the rest of the draft")
		}
		if err := s.releaseRepo.UpdateNotes(ctx, release.IdRelease, notes); err != nil {
			return err
		}
		release.Notes = notes
		updated = release
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *ReleaseService) NotesWritten(ctx context.Context, idRelease int64, notes string) error {
	release, err := s.releaseRepo.SetDraftNotes(ctx, idRelease, notes)
	if err != nil || release == nil {
		return err
	}
	extctx.AfterCommit(ctx, func(detached context.Context) {
		if s.onReleaseChanged != nil {
			s.onReleaseChanged(detached, release.IdProject, release.IdRelease)
		}
	})
	return nil
}

func (s *ReleaseService) lockDraft(ctx context.Context, idProject, idRelease int64) (*model.Release, error) {
	release, err := s.releaseRepo.LockRelease(ctx, idProject, idRelease)
	if err != nil {
		return nil, err
	}
	if release.Status != constants.ReleaseStatusDraft {
		return nil, errs.ErrReleaseNotDraft
	}
	return release, nil
}

func (s *ReleaseService) draftTag(ctx context.Context, release *model.Release) (*model.Tag, error) {
	if release.IdTag == nil {
		return nil, errs.ErrTagNotInProject.WithMessage("the tag of this release was deleted")
	}
	tag, err := s.tagRepo.LoadTag(ctx, release.IdProject, *release.IdTag)
	if err != nil {
		return nil, tagErr(err)
	}
	return tag, nil
}

func (s *ReleaseService) checkNoActiveRun(ctx context.Context, release *model.Release) error {
	run, err := s.agentRunRepo.LoadActiveByRelease(ctx, release.IdRelease)
	if err != nil {
		return err
	}
	if run != nil {
		return errs.ErrReleaseNotesPending
	}
	return nil
}

func (s *ReleaseService) checkNameFree(ctx context.Context, idProject, idRelease int64, source *model.Tag, name string) error {
	taken, err := s.releaseRepo.IsNameTaken(ctx, idProject, idRelease, name)
	if err != nil {
		return err
	}
	if taken {
		return errs.ErrReleaseNameTaken
	}
	tags, err := s.tagRepo.LoadTags(ctx, []int64{idProject})
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if tag.IdTag != source.IdTag && strings.EqualFold(tag.Name, name) {
			return errs.ErrReleaseNameTaken
		}
	}
	return nil
}

func (s *ReleaseService) startNotesRun(ctx context.Context, release *model.Release, req model.StartReleaseAgentReq, result *ReleaseResult) error {
	agentUser, err := s.userRepo.LoadUser(ctx, req.IdUserAgent)
	if err != nil || agentUser == nil {
		return errs.ErrNotFound.WithMessage("agent not found")
	}
	if !agentUser.IsAgent {
		return errs.ErrValidation.WithMessage("the user is not an agent")
	}
	if !s.acl.CanReadProject(ctx, agentUser.IdUser, release.IdProject) {
		return errs.ErrForbidden
	}
	gateway, err := s.agentGwRepo.LoadByAgentUser(ctx, agentUser.IdUser)
	if err != nil || gateway == nil {
		return errs.ErrAgentNoGateway
	}
	choices := req.RunChoices
	choices.IsShortWorkflow = false
	if choices.IdsSkillByStage == nil {
		defaults, err := s.projectSkillService.LoadDefaultIdsSkillByStage(ctx, release.IdProject)
		if err != nil {
			return err
		}
		choices.IdsSkillByStage = map[string][]int64{constants.StageReleaseNotes: defaults[constants.StageReleaseNotes]}
	}
	if err := s.agentTaskConfig.ValidateChoices(gateway, choices); err != nil {
		return err
	}

	stagePlan, err := s.stagePlan.BuildFromChoices(constants.RunKindReleaseNotes, choices)
	if err != nil {
		return err
	}
	run, err := s.agentRunRepo.InsertForRelease(ctx, release.IdRelease, agentUser.IdUser, release.IdProject, stagePlan)
	if err != nil {
		return err
	}
	result.Run = run
	return nil
}

func (s *ReleaseService) Delete(ctx context.Context, user model.User, idProject, idRelease int64, req model.DeleteReleaseReq) (*ReleaseResult, error) {
	result := &ReleaseResult{}
	err := extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		release, err := s.releaseRepo.LockRelease(ctx, idProject, idRelease)
		if err != nil {
			return err
		}
		result.Release = release
		if release.Status == constants.ReleaseStatusReleased {
			if err := s.settleTag(ctx, release, req, result); err != nil {
				return err
			}
		}
		if err := s.stopNotesRun(ctx, user, release, result); err != nil {
			return err
		}
		if err := s.agentRunRepo.DeleteByRelease(ctx, release.IdRelease); err != nil {
			return err
		}
		return s.releaseRepo.DeleteRelease(ctx, release.IdRelease)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *ReleaseService) settleTag(ctx context.Context, release *model.Release, req model.DeleteReleaseReq, result *ReleaseResult) error {
	if req.Tag == "" || req.Tag == constants.ReleaseTagKeep {
		return nil
	}
	if req.IdTagTarget == nil {
		return errs.ErrValidation.WithMessage("choose the tag to move the tasks to")
	}
	if release.IdTag != nil && *req.IdTagTarget == *release.IdTag {
		return errs.ErrValidation.WithMessage("the tasks must move to another tag")
	}
	if _, err := s.tagRepo.LoadTag(ctx, release.IdProject, *req.IdTagTarget); err != nil {
		return tagErr(err)
	}
	if release.IdTag == nil {
		return nil
	}
	versionTag, err := s.tagRepo.LoadTag(ctx, release.IdProject, *release.IdTag)
	if err != nil {
		return err
	}
	idsIssue, err := s.releaseRepo.LoadTagIssueIds(ctx, versionTag.IdTag)
	if err != nil {
		return err
	}
	if err := s.issueRepo.AddIssueTags(ctx, idsIssue, []int64{*req.IdTagTarget}); err != nil {
		return err
	}
	result.IdsIssueChanged = idsIssue
	if req.Tag == constants.ReleaseTagMoveDelete {
		if err := s.tagRepo.DeleteTag(ctx, release.IdProject, versionTag.IdTag); err != nil {
			return err
		}
		result.DeletedTag = versionTag
		return nil
	}
	if err := s.issueRepo.RemoveIssueTags(ctx, idsIssue, []int64{versionTag.IdTag}); err != nil {
		return err
	}
	versionTag.IssueCount = 0
	result.RenamedTag = versionTag
	return nil
}

func (s *ReleaseService) stopNotesRun(ctx context.Context, user model.User, release *model.Release, result *ReleaseResult) error {
	run, err := s.agentRunRepo.LoadActiveByRelease(ctx, release.IdRelease)
	if err != nil || run == nil {
		return err
	}
	cancelled, err := s.agentRunRepo.TransitionPhase(ctx, run.IdRun, run.Phase, constants.PhaseCancelled, constants.ActorTypeUser, &user.IdUser, "release deleted")
	if err != nil {
		return err
	}
	idsTask, err := s.agentTaskRepo.CancelNonTerminalForRun(ctx, run.IdRun)
	if err != nil {
		return err
	}
	result.CancelledRun = cancelled
	result.IdsTaskStopped = idsTask
	return nil
}

func (s *ReleaseService) PromptContext(ctx context.Context, run *model.AgentRun) (*model.ReleasePromptContext, error) {
	if run.IdRelease == nil {
		return nil, fmt.Errorf("run %d writes no release notes", run.IdRun)
	}
	release, err := s.releaseRepo.LoadById(ctx, *run.IdRelease)
	if err != nil {
		return nil, err
	}
	prompt := &model.ReleasePromptContext{Name: release.Name, Sections: []*model.ReleaseSection{}, OpenIssues: []*model.ReleaseIssue{}}
	if release.IdTag == nil {
		return prompt, nil
	}
	tag, err := s.tagRepo.LoadTag(ctx, release.IdProject, *release.IdTag)
	if err != nil {
		return nil, tagErr(err)
	}
	issues, err := s.releaseRepo.LoadTagIssues(ctx, release.IdProject, *release.IdTag)
	if err != nil {
		return nil, err
	}
	draft := buildReleaseDraft(issues)
	prompt.Tag = tag.Name
	prompt.Sections = draft.Sections
	if release.OpenIssues == constants.ReleaseOpenIssuesKeep {
		prompt.OpenIssues = draft.OpenIssues
	}
	return prompt, nil
}

func releaseName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || wikitext.Slugify(name) == "" {
		return "", errs.ErrValidation.WithMessage("the version name needs at least one letter or digit")
	}
	return name, nil
}

func tagErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errs.ErrTagNotInProject
	}
	return err
}
