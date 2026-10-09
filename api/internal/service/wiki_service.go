package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/lexorank"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikitext"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	wikiSearchLimit     = 20
	wikiListPageSize    = 20
	wikiListMaxPageSize = 100
)

type WikiService struct {
	pool          *pgxpool.Pool
	spaceRepo     *repository.WikiSpaceRepository
	pageRepo      *repository.WikiPageRepository
	versionRepo   *repository.WikiPageVersionRepository
	draftRepo     *repository.WikiPageDraftRepository
	linkRepo      *repository.WikiPageLinkRepository
	issuePageRepo *repository.WikiIssuePageRepository
	proposalRepo  *repository.WikiProposalRepository
	projectRepo   *repository.ProjectRepository
	acl           *AclService
	notifier      *notify.Notifier
	cache         *redis.Client
}

type wikiAccess struct {
	space     *model.WikiSpace
	canRead   bool
	canEdit   bool
	canManage bool
}

func NewWikiService(
	pool *pgxpool.Pool,
	spaceRepo *repository.WikiSpaceRepository,
	pageRepo *repository.WikiPageRepository,
	versionRepo *repository.WikiPageVersionRepository,
	draftRepo *repository.WikiPageDraftRepository,
	linkRepo *repository.WikiPageLinkRepository,
	issuePageRepo *repository.WikiIssuePageRepository,
	proposalRepo *repository.WikiProposalRepository,
	projectRepo *repository.ProjectRepository,
	acl *AclService,
	notifier *notify.Notifier,
	cache *redis.Client,
) *WikiService {
	return &WikiService{
		pool:          pool,
		spaceRepo:     spaceRepo,
		pageRepo:      pageRepo,
		versionRepo:   versionRepo,
		draftRepo:     draftRepo,
		linkRepo:      linkRepo,
		issuePageRepo: issuePageRepo,
		proposalRepo:  proposalRepo,
		projectRepo:   projectRepo,
		acl:           acl,
		notifier:      notifier,
		cache:         cache,
	}
}

func (s *WikiService) LoadTree(ctx context.Context, user model.User, idProject int64) (*model.WikiTree, error) {
	instance, project, err := s.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	nodes, err := s.pageRepo.LoadTree(ctx, []int64{instance.IdSpace, project.IdSpace}, user.IsAgent)
	if err != nil {
		return nil, err
	}
	alwaysChars, err := s.pageRepo.SumAlwaysChars(ctx, []int64{instance.IdSpace, project.IdSpace}, 0)
	if err != nil {
		return nil, err
	}
	budgets, err := s.spaceRepo.LoadProjectBudgets(ctx, []int64{idProject})
	if err != nil {
		return nil, err
	}
	limit := 0
	if len(budgets) > 0 {
		limit = budgets[0].Limit
	}
	instanceAccess := s.spaceAccess(ctx, user, instance)
	projectAccess := s.spaceAccess(ctx, user, project)
	idHomePage, err := s.spaceRepo.LoadHomePage(ctx, project.IdSpace)
	if err != nil {
		return nil, err
	}
	trashCount, err := s.pageRepo.CountTrash(ctx, s.trashSpaceIds(ctx, user, instance, project), user.IsAgent)
	if err != nil {
		return nil, err
	}
	proposalCount, err := s.proposalRepo.CountReady(ctx, idProject)
	if err != nil {
		return nil, err
	}
	return &model.WikiTree{
		Spaces: []model.WikiSpaceView{
			{IdSpace: instance.IdSpace, Kind: instance.Kind, CanEdit: instanceAccess.canEdit, CanManage: instanceAccess.canManage},
			{IdSpace: project.IdSpace, Kind: project.Kind, CanEdit: projectAccess.canEdit, CanManage: projectAccess.canManage, IdHomePage: idHomePage},
		},
		Nodes:         nodes,
		AlwaysTokens:  tokensOfChars(alwaysChars),
		TokenLimit:    limit,
		TrashCount:    trashCount,
		ProposalCount: proposalCount,
	}, nil
}

func (s *WikiService) LoadPage(ctx context.Context, user model.User, idProject int64, kind constants.WikiSpaceKind, slug string) (*model.WikiPageView, error) {
	instance, project, err := s.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	space := s.spaceByKind(instance, project, kind)
	page, err := s.pageRepo.LoadLivePageBySlug(ctx, space.IdSpace, slug)
	if err != nil {
		return nil, err
	}
	if page == nil || isHiddenFrom(user, page) {
		return nil, errs.ErrNotFound
	}
	access := s.spaceAccess(ctx, user, space)
	backlinks, err := s.loadBacklinks(ctx, user, page, []int64{instance.IdSpace, project.IdSpace}, wikiListPageSize, 0)
	if err != nil {
		return nil, err
	}
	issues, err := s.loadPageIssues(ctx, user, page.IdPage, wikiListPageSize, 0)
	if err != nil {
		return nil, err
	}
	links, err := s.resolveLinks(ctx, instance, project, space, page.Body, user.IsAgent)
	if err != nil {
		return nil, err
	}
	draft, err := s.draftRepo.Load(ctx, page.IdPage, user.IdUser)
	if err != nil {
		return nil, err
	}
	return &model.WikiPageView{
		Page:      page,
		SpaceKind: space.Kind,
		CanEdit:   access.canEdit,
		CanManage: access.canManage,
		Backlinks: *backlinks,
		Issues:    *issues,
		Links:     links,
		Draft:     draft,
	}, nil
}

func (s *WikiService) Create(ctx context.Context, user model.User, idProject int64, req model.CreateWikiPageReq) (*model.WikiPage, error) {
	slug := wikitext.Slugify(req.Title)
	if slug == "" {
		return nil, errs.ErrWikiInvalidTitle
	}
	agentAccess := req.AgentAccess
	if agentAccess == "" {
		agentAccess = constants.WikiAgentAccessOnDemand
	}
	var created *model.WikiPage
	err := extctx.JoinTx(ctx, s.pool, func(ctx context.Context) error {
		instance, project, err := s.projectSpaces(ctx, user, idProject)
		if err != nil {
			return err
		}
		space := s.spaceByKind(instance, project, req.Space)
		access := s.spaceAccess(ctx, user, space)
		if !access.canEdit {
			return errs.ErrForbidden
		}
		if agentAccess == constants.WikiAgentAccessAlways && !access.canManage {
			return errs.ErrForbidden
		}
		if err := s.spaceRepo.LockSpace(ctx, space.IdSpace); err != nil {
			return err
		}
		if err := s.validateParent(ctx, space.IdSpace, req.IdParent, 0); err != nil {
			return err
		}
		taken, err := s.pageRepo.IsSlugTaken(ctx, space.IdSpace, slug)
		if err != nil {
			return err
		}
		if taken {
			return errs.ErrWikiSlugTaken
		}
		if agentAccess == constants.WikiAgentAccessAlways {
			if err := s.checkBudget(ctx, space, nil, wikiChars(req.Title, req.Body), 0); err != nil {
				return err
			}
		}
		lastRank, err := s.pageRepo.LoadLastSiblingRank(ctx, space.IdSpace, req.IdParent)
		if err != nil {
			return err
		}
		prev := ""
		if lastRank != nil {
			prev = *lastRank
		}
		page, err := s.pageRepo.InsertPage(ctx, &model.WikiPage{
			IdSpace:     space.IdSpace,
			IdParent:    req.IdParent,
			Slug:        slug,
			Title:       req.Title,
			Summary:     req.Summary,
			Body:        req.Body,
			AgentAccess: agentAccess,
			Rank:        lexorank.Between(prev, ""),
		}, user.IdUser)
		if err != nil {
			return err
		}
		if err := s.versionRepo.Insert(ctx, versionOf(page, req.Note, nil, user.IdUser)); err != nil {
			return err
		}
		if err := s.storeLinks(ctx, page, space); err != nil {
			return err
		}
		created = page
		s.notifySaved(ctx, space, page, user)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *WikiService) Search(ctx context.Context, user model.User, idProject int64, query string) ([]*model.WikiSearchHit, error) {
	instance, project, err := s.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	prefixQuery := wikitext.ToPrefixQuery(query)
	if prefixQuery == "" {
		return []*model.WikiSearchHit{}, nil
	}
	return s.pageRepo.Search(ctx, []int64{instance.IdSpace, project.IdSpace}, prefixQuery, wikiSearchLimit, user.IsAgent)
}

func (s *WikiService) LoadPageIssues(ctx context.Context, user model.User, idPage int64, limit, offset int) (*model.WikiPageIssueList, error) {
	if _, _, err := s.loadPageAccess(ctx, user, idPage, false); err != nil {
		return nil, err
	}
	return s.loadPageIssues(ctx, user, idPage, clampListPageSize(limit), max(offset, 0))
}

func (s *WikiService) LoadPageBacklinks(ctx context.Context, user model.User, idProject, idPage int64, limit, offset int) (*model.WikiBacklinkList, error) {
	instance, project, err := s.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	page, _, err := s.loadPageAccess(ctx, user, idPage, false)
	if err != nil {
		return nil, err
	}
	if page.DeletedAt != nil || (page.IdSpace != instance.IdSpace && page.IdSpace != project.IdSpace) {
		return nil, errs.ErrNotFound
	}
	return s.loadBacklinks(ctx, user, page, []int64{instance.IdSpace, project.IdSpace}, clampListPageSize(limit), max(offset, 0))
}

func (s *WikiService) UpdateSettings(ctx context.Context, user model.User, idProject int64, req model.WikiSettingsReq) error {
	if user.IsAgent || !s.acl.CanManageWiki(ctx, user.IdUser, idProject) {
		return errs.ErrForbidden
	}
	return s.spaceRepo.UpdateTokenLimit(ctx, idProject, req.AlwaysTokenLimit)
}

func (s *WikiService) UpdateHomePage(ctx context.Context, user model.User, idProject int64, req model.WikiHomeReq) error {
	if user.IsAgent || !s.acl.CanManageWiki(ctx, user.IdUser, idProject) {
		return errs.ErrForbidden
	}
	return extctx.RunInTx(ctx, s.pool, func(ctx context.Context) error {
		_, project, err := s.projectSpaces(ctx, user, idProject)
		if err != nil {
			return err
		}
		if req.IdPage != nil {
			page, err := s.pageRepo.LoadPage(ctx, *req.IdPage)
			if err != nil {
				return err
			}
			if page == nil || page.DeletedAt != nil || page.IdSpace != project.IdSpace {
				return errs.ErrBadRequest.WithMessage("the wiki home must be a live page of this project")
			}
		}
		return s.spaceRepo.UpdateHomePage(ctx, project.IdSpace, req.IdPage)
	})
}

func (s *WikiService) LoadIssueLinks(ctx context.Context, user model.User, idIssue int64) ([]model.WikiIssueLink, error) {
	project, err := s.projectOfIssue(ctx, idIssue)
	if err != nil {
		return nil, err
	}
	if !s.acl.CanReadWiki(ctx, user.IdUser, project.IdProject) {
		return nil, errs.ErrNotFound
	}
	instance, err := s.spaceRepo.LoadInstanceSpace(ctx)
	if err != nil {
		return nil, err
	}
	refs, err := s.issuePageRepo.LoadByIssue(ctx, idIssue, user.IsAgent)
	if err != nil {
		return nil, err
	}
	links := make([]model.WikiIssueLink, len(refs))
	for i, ref := range refs {
		kind := constants.WikiSpaceProject
		if ref.IdSpace == instance.IdSpace {
			kind = constants.WikiSpaceInstance
		}
		links[i] = model.WikiIssueLink{
			IdPage: ref.IdPage, IdSpace: ref.IdSpace, SpaceKind: kind,
			Slug: ref.Slug, Title: ref.Title, Source: ref.Source,
		}
	}
	return links, nil
}

func (s *WikiService) AddIssueLink(ctx context.Context, user model.User, idIssue, idPage int64) error {
	project, err := s.projectOfIssue(ctx, idIssue)
	if err != nil {
		return err
	}
	if !s.acl.CanUpdateIssue(ctx, user.IdUser, project.IdProject) {
		return errs.ErrForbidden
	}
	page, err := s.pageRepo.LoadPage(ctx, idPage)
	if err != nil {
		return err
	}
	if page == nil || page.DeletedAt != nil || isHiddenFrom(user, page) {
		return errs.ErrNotFound
	}
	instance, projectSpace, err := s.projectSpaces(ctx, user, project.IdProject)
	if err != nil {
		return err
	}
	if page.IdSpace != instance.IdSpace && page.IdSpace != projectSpace.IdSpace {
		return errs.ErrNotFound
	}
	return s.issuePageRepo.Insert(ctx, idIssue, idPage, constants.WikiIssueLinkManual)
}

func (s *WikiService) RemoveIssueLink(ctx context.Context, user model.User, idIssue, idPage int64) error {
	project, err := s.projectOfIssue(ctx, idIssue)
	if err != nil {
		return err
	}
	if !s.acl.CanUpdateIssue(ctx, user.IdUser, project.IdProject) {
		return errs.ErrForbidden
	}
	return s.issuePageRepo.Delete(ctx, idIssue, idPage)
}

func (s *WikiService) SyncIssueDescription(ctx context.Context, idIssue, idProject int64, description string) error {
	parsed := wikitext.ParseLinks(description)
	if len(parsed) == 0 {
		return s.issuePageRepo.ReplaceFromDescription(ctx, idIssue, []int64{})
	}
	instance, err := s.spaceRepo.LoadInstanceSpace(ctx)
	if err != nil {
		return err
	}
	project, err := s.spaceRepo.EnsureProjectSpace(ctx, idProject)
	if err != nil {
		return err
	}
	resolved, err := s.resolveLinks(ctx, instance, project, project, description, false)
	if err != nil {
		return err
	}
	idsPage := make([]int64, 0, len(resolved))
	for _, link := range resolved {
		if link.IdPage != nil {
			idsPage = append(idsPage, *link.IdPage)
		}
	}
	return s.issuePageRepo.ReplaceFromDescription(ctx, idIssue, idsPage)
}

func (s *WikiService) loadPageIssues(ctx context.Context, user model.User, idPage int64, limit, offset int) (*model.WikiPageIssueList, error) {
	visibleProjects, err := s.acl.LoadVisibleProjectIds(ctx, user.IdUser)
	if err != nil {
		return nil, err
	}
	items, err := s.issuePageRepo.LoadByPage(ctx, idPage, visibleProjects, limit, offset)
	if err != nil {
		return nil, err
	}
	total, err := s.issuePageRepo.CountByPage(ctx, idPage, visibleProjects)
	if err != nil {
		return nil, err
	}
	return &model.WikiPageIssueList{Items: items, Total: total}, nil
}

func (s *WikiService) loadBacklinks(ctx context.Context, user model.User, page *model.WikiPage, idsSourceSpace []int64, limit, offset int) (*model.WikiBacklinkList, error) {
	items, err := s.linkRepo.LoadBacklinks(ctx, page.IdSpace, page.Slug, idsSourceSpace, user.IsAgent, limit, offset)
	if err != nil {
		return nil, err
	}
	total, err := s.linkRepo.CountBacklinks(ctx, page.IdSpace, page.Slug, idsSourceSpace, user.IsAgent)
	if err != nil {
		return nil, err
	}
	return &model.WikiBacklinkList{Items: items, Total: total}, nil
}

func (s *WikiService) spaceAccess(ctx context.Context, user model.User, space *model.WikiSpace) wikiAccess {
	access := wikiAccess{space: space}
	if space.Kind == constants.WikiSpaceInstance {
		access.canRead = true
		access.canEdit = !user.IsAgent && s.acl.CanEditSharedWiki(ctx, user.IdUser)
		access.canManage = !user.IsAgent && s.acl.CanManageSharedWiki(ctx, user.IdUser)
		return access
	}
	access.canRead = s.acl.CanReadWiki(ctx, user.IdUser, *space.IdProject)
	access.canEdit = !user.IsAgent && s.acl.CanEditWiki(ctx, user.IdUser, *space.IdProject)
	access.canManage = !user.IsAgent && s.acl.CanManageWiki(ctx, user.IdUser, *space.IdProject)
	return access
}

func (s *WikiService) projectSpaces(ctx context.Context, user model.User, idProject int64) (*model.WikiSpace, *model.WikiSpace, error) {
	if !s.acl.CanReadWiki(ctx, user.IdUser, idProject) {
		return nil, nil, errs.ErrForbidden
	}
	instance, err := s.spaceRepo.LoadInstanceSpace(ctx)
	if err != nil {
		return nil, nil, err
	}
	project, err := s.spaceRepo.EnsureProjectSpace(ctx, idProject)
	if err != nil {
		return nil, nil, err
	}
	return instance, project, nil
}

func (s *WikiService) spaceByKind(instance, project *model.WikiSpace, kind constants.WikiSpaceKind) *model.WikiSpace {
	if kind == constants.WikiSpaceInstance {
		return instance
	}
	return project
}

func (s *WikiService) loadPageAccess(ctx context.Context, user model.User, idPage int64, lock bool) (*model.WikiPage, wikiAccess, error) {
	var page *model.WikiPage
	var err error
	if lock {
		page, err = s.pageRepo.LockPage(ctx, idPage)
	} else {
		page, err = s.pageRepo.LoadPage(ctx, idPage)
	}
	if err != nil {
		return nil, wikiAccess{}, err
	}
	if page == nil {
		return nil, wikiAccess{}, errs.ErrNotFound
	}
	space, err := s.spaceRepo.LoadSpace(ctx, page.IdSpace)
	if err != nil {
		return nil, wikiAccess{}, err
	}
	if space == nil {
		return nil, wikiAccess{}, errs.ErrNotFound
	}
	access := s.spaceAccess(ctx, user, space)
	if !access.canRead || isHiddenFrom(user, page) {
		return nil, wikiAccess{}, errs.ErrNotFound
	}
	return page, access, nil
}

func isHiddenFrom(user model.User, page *model.WikiPage) bool {
	return user.IsAgent && page.AgentAccess == constants.WikiAgentAccessHidden
}

func (s *WikiService) resolveLinks(ctx context.Context, instance, project, current *model.WikiSpace, body string, hideHidden bool) ([]model.WikiResolvedLink, error) {
	parsed := wikitext.ParseLinks(body)
	resolved := make([]model.WikiResolvedLink, 0, len(parsed))
	if len(parsed) == 0 {
		return resolved, nil
	}
	slugs := make([]string, len(parsed))
	for i, link := range parsed {
		slugs[i] = link.Slug
	}
	idsSpace := []int64{instance.IdSpace}
	if project != nil {
		idsSpace = append(idsSpace, project.IdSpace)
	}
	refs, err := s.pageRepo.LoadLivePagesBySlugs(ctx, idsSpace, slugs, hideHidden)
	if err != nil {
		return nil, err
	}
	bySpaceSlug := map[string]*model.WikiPageRef{}
	for _, ref := range refs {
		bySpaceSlug[strconv.FormatInt(ref.IdSpace, 10)+"/"+ref.Slug] = ref
	}
	for _, link := range parsed {
		target := s.linkTarget(link, instance, current, bySpaceSlug)
		entry := model.WikiResolvedLink{Shared: link.Shared, Slug: link.Slug, IdSpace: target.IdSpace}
		if ref, ok := bySpaceSlug[strconv.FormatInt(target.IdSpace, 10)+"/"+link.Slug]; ok {
			entry.IdPage = &ref.IdPage
			entry.Title = ref.Title
		}
		resolved = append(resolved, entry)
	}
	return resolved, nil
}

func (s *WikiService) linkTarget(link wikitext.Link, instance, current *model.WikiSpace, existing map[string]*model.WikiPageRef) *model.WikiSpace {
	if link.Shared || current.IdSpace == instance.IdSpace {
		return instance
	}
	if _, ok := existing[strconv.FormatInt(current.IdSpace, 10)+"/"+link.Slug]; ok {
		return current
	}
	if _, ok := existing[strconv.FormatInt(instance.IdSpace, 10)+"/"+link.Slug]; ok {
		return instance
	}
	return current
}

func (s *WikiService) storeLinks(ctx context.Context, page *model.WikiPage, current *model.WikiSpace) error {
	instance, err := s.spaceRepo.LoadInstanceSpace(ctx)
	if err != nil {
		return err
	}
	var project *model.WikiSpace
	if current.Kind == constants.WikiSpaceProject {
		project = current
	}
	resolved, err := s.resolveLinks(ctx, instance, project, current, page.Body, false)
	if err != nil {
		return err
	}
	links := make([]model.WikiLink, len(resolved))
	for i, link := range resolved {
		links[i] = model.WikiLink{IdSpaceTo: link.IdSpace, TargetSlug: link.Slug}
	}
	return s.linkRepo.Replace(ctx, page.IdPage, links)
}

func (s *WikiService) validateParent(ctx context.Context, idSpace int64, idParent *int64, idPage int64) error {
	if idParent == nil {
		return nil
	}
	parent, err := s.pageRepo.LoadPage(ctx, *idParent)
	if err != nil {
		return err
	}
	if parent == nil || parent.IdSpace != idSpace || parent.DeletedAt != nil {
		return errs.ErrWikiInvalidParent
	}
	if idPage == 0 {
		return nil
	}
	ancestors, err := s.pageRepo.LoadLiveAncestorIds(ctx, *idParent)
	if err != nil {
		return err
	}
	for _, idAncestor := range ancestors {
		if idAncestor == idPage {
			return errs.ErrWikiInvalidParent
		}
	}
	return nil
}

func (s *WikiService) projectOfIssue(ctx context.Context, idIssue int64) (*model.Project, error) {
	project, err := s.projectRepo.LoadProjectByIssue(ctx, idIssue)
	if isNoRows(err) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (s *WikiService) notifySaved(ctx context.Context, space *model.WikiSpace, page *model.WikiPage, user model.User) {
	if s.notifier == nil {
		return
	}
	notice := &notify.Notice{
		Subject: notify.SubjectWikiPage,
		Action:  notify.ActionUpdate,
		Payload: model.WikiSavedNotice{
			IdPage: page.IdPage, IdSpace: page.IdSpace, VersionNo: page.VersionNo,
			IdUser: user.IdUser, UserName: user.Name,
		},
	}
	extctx.AfterCommit(ctx, func(ctx context.Context) {
		if space.Kind == constants.WikiSpaceProject {
			members, err := s.projectRepo.LoadProjectsMembers(ctx, []int64{*space.IdProject})
			if err != nil {
				extctx.GetLogger(ctx).Error().Err(err).Msg("wiki: loading members for saved notice")
				return
			}
			if len(members) == 0 {
				return
			}
			notice.IdsUser = make([]int64, len(members))
			for i, member := range members {
				notice.IdsUser[i] = member.IdUser
			}
		}
		s.notifier.Send <- notice
	})
}

func versionOf(page *model.WikiPage, note string, mergedFrom *int, idUser int64) *model.WikiPageVersion {
	return &model.WikiPageVersion{
		IdPage:      page.IdPage,
		VersionNo:   page.VersionNo,
		IdParent:    page.IdParent,
		Title:       page.Title,
		Summary:     page.Summary,
		Body:        page.Body,
		AgentAccess: page.AgentAccess,
		Note:        note,
		MergedFrom:  mergedFrom,
		CreateBy:    &idUser,
	}
}

func clampListPageSize(limit int) int {
	if limit <= 0 {
		return wikiListPageSize
	}
	return min(limit, wikiListMaxPageSize)
}

func tokensOfChars(chars int) int {
	return (chars + 3) / 4
}

func wikiChars(title, body string) int {
	return len([]rune(title)) + len([]rune(body))
}

func (s *WikiService) checkBudget(ctx context.Context, space *model.WikiSpace, idsExcluded []int64, ownChars, previousChars int) error {
	if ownChars <= previousChars {
		return nil
	}
	if err := s.spaceRepo.LockBudget(ctx); err != nil {
		return err
	}
	instance, err := s.spaceRepo.LoadInstanceSpace(ctx)
	if err != nil {
		return err
	}
	sums, err := s.pageRepo.SumAlwaysCharsBySpace(ctx, idsExcluded)
	if err != nil {
		return err
	}
	var idsProject []int64
	if space.Kind == constants.WikiSpaceProject {
		idsProject = []int64{*space.IdProject}
	}
	budgets, err := s.spaceRepo.LoadProjectBudgets(ctx, idsProject)
	if err != nil {
		return err
	}
	for _, budget := range budgets {
		if space.Kind == constants.WikiSpaceInstance && budget.Limit == 0 {
			continue
		}
		used := sums[instance.IdSpace] + ownChars
		if budget.IdSpace != nil {
			used += sums[*budget.IdSpace]
		}
		if tokensOfChars(used) > budget.Limit {
			return errs.ErrWikiTokenLimit.WithMessage(fmt.Sprintf("always-read wiki pages would use %d of %d tokens", tokensOfChars(used), budget.Limit))
		}
	}
	return nil
}
