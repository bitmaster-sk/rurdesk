package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikitext"
	"github.com/google/uuid"
)

const (
	wikiAgentSearchDefaultLimit = 10
	wikiAgentSearchMaxLimit     = 50
)

type WikiAgentService struct {
	wiki     *WikiService
	readRepo *repository.AgentRunWikiReadRepository
	runRepo  *repository.AgentRunRepository
	taskRepo *repository.AgentTaskRepository
}

type wikiRunRef struct {
	idRun  int64
	idTask int64
	stage  string
}

func NewWikiAgentService(
	wiki *WikiService,
	readRepo *repository.AgentRunWikiReadRepository,
	runRepo *repository.AgentRunRepository,
	taskRepo *repository.AgentTaskRepository,
) *WikiAgentService {
	return &WikiAgentService{wiki: wiki, readRepo: readRepo, runRepo: runRepo, taskRepo: taskRepo}
}

func (s *WikiAgentService) BuildPromptContext(ctx context.Context, run *model.AgentRun, task *model.AgentTask) (*model.WikiPromptContext, []*model.AgentRunWikiRead, error) {
	instance, err := s.wiki.spaceRepo.LoadInstanceSpace(ctx)
	if err != nil {
		return nil, nil, err
	}
	project, err := s.wiki.spaceRepo.EnsureProjectSpace(ctx, run.IdProject)
	if err != nil {
		return nil, nil, err
	}
	linked, err := s.wiki.issuePageRepo.LoadByIssue(ctx, run.IdIssue, true)
	if err != nil {
		return nil, nil, err
	}
	idsLinked := make(map[int64]bool, len(linked))
	idsWithBody := make([]int64, 0, len(linked))
	for _, ref := range linked {
		idsLinked[ref.IdPage] = true
		idsWithBody = append(idsWithBody, ref.IdPage)
	}
	pages, err := s.wiki.pageRepo.LoadAgentPages(ctx, []int64{instance.IdSpace, project.IdSpace}, idsWithBody)
	if err != nil {
		return nil, nil, err
	}
	budgets, err := s.wiki.spaceRepo.LoadProjectBudgets(ctx, []int64{run.IdProject})
	if err != nil {
		return nil, nil, err
	}
	limit := 0
	if len(budgets) > 0 {
		limit = budgets[0].Limit
	}

	prompt, promptReads := buildWikiPrompt(wikiPromptInput{
		Pages:           pages,
		IdsLinked:       idsLinked,
		IdInstanceSpace: instance.IdSpace,
		TokenLimit:      limit,
	})
	if len(prompt.Pages) == 0 && len(prompt.Index) == 0 {
		return nil, nil, nil
	}
	ref := &wikiRunRef{idRun: run.IdRun, idTask: task.IdTask, stage: task.Stage}
	idCall := uuid.New()
	reads := make([]*model.AgentRunWikiRead, 0, len(promptReads))
	var index *model.AgentRunWikiRead
	for _, read := range promptReads {
		kind := spaceKindOf(read.Page.IdSpace, instance)
		if read.Source != constants.WikiReadPromptIndex {
			reads = append(reads, ref.read(idCall, read.Source, read.Page.IdPage, kind, read.Page.Slug, read.Page.Title, read.Page.VersionNo, read.Tokens))
			continue
		}
		if index == nil {
			index = &model.AgentRunWikiRead{
				IdRun: ref.idRun, IdTask: &ref.idTask, Stage: ref.stage, IdCall: idCall,
				Source: constants.WikiReadPromptIndex, Pages: []model.WikiReadIndexPage{},
			}
			reads = append(reads, index)
		}
		index.Tokens += read.Tokens
		index.Pages = append(index.Pages, model.WikiReadIndexPage{
			IdPage: read.Page.IdPage, SpaceKind: kind, Slug: read.Page.Slug, Title: read.Page.Title,
		})
	}
	return &prompt, reads, nil
}

func (s *WikiAgentService) RecordReads(ctx context.Context, reads []*model.AgentRunWikiRead) error {
	return s.readRepo.Insert(ctx, reads)
}

func (s *WikiAgentService) LoadRunReads(ctx context.Context, user model.User, idRun int64) ([]*model.AgentRunWikiRead, error) {
	run, err := s.runRepo.LoadById(ctx, idRun)
	if errors.Is(err, repository.ErrRunNotFound) {
		return nil, errs.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !s.wiki.acl.CanReadProject(ctx, user.IdUser, run.IdProject) {
		return nil, errs.ErrForbidden
	}
	return s.readRepo.LoadByRun(ctx, idRun)
}

func (s *WikiAgentService) Search(ctx context.Context, user model.User, idProject int64, query string, limit int, idRun int64) ([]model.WikiAgentSearchHit, error) {
	instance, project, err := s.wiki.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	hits := []model.WikiAgentSearchHit{}
	prefixQuery := wikitext.ToPrefixQuery(query)
	if prefixQuery == "" {
		return hits, nil
	}
	found, err := s.wiki.pageRepo.Search(ctx, []int64{instance.IdSpace, project.IdSpace}, prefixQuery, clampSearchLimit(limit), true)
	if err != nil {
		return nil, err
	}
	ref := s.runRef(ctx, user, idRun, idProject)
	idCall := uuid.New()
	reads := make([]*model.AgentRunWikiRead, 0, len(found))
	for _, hit := range found {
		kind := spaceKindOf(hit.IdSpace, instance)
		entry := model.WikiAgentSearchHit{
			Slug:    agentSlugIn(kind, hit.Slug),
			Space:   kind,
			Title:   hit.Title,
			Summary: hit.Summary,
			Snippet: hit.Snippet,
			Version: hit.VersionNo,
		}
		hits = append(hits, entry)
		if ref != nil {
			read := ref.read(idCall, constants.WikiReadMcpSearch, hit.IdPage, kind, hit.Slug, hit.Title, hit.VersionNo,
				wikitext.EstimateTokens(entry.Title+entry.Summary+entry.Snippet))
			read.Query = &query
			reads = append(reads, read)
		}
	}
	if ref != nil && len(reads) == 0 {
		reads = append(reads, &model.AgentRunWikiRead{
			IdRun: ref.idRun, IdTask: &ref.idTask, Stage: ref.stage, IdCall: idCall,
			Source: constants.WikiReadMcpSearch, Query: &query,
		})
	}
	s.recordQuietly(ctx, reads)
	return hits, nil
}

func (s *WikiAgentService) LoadPage(ctx context.Context, user model.User, idProject int64, target string, versionNo int, idRun int64) (*model.WikiAgentPage, error) {
	link, ok := wikitext.ParseTarget(target)
	if !ok {
		return nil, errs.ErrNotFound
	}
	instance, project, err := s.wiki.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	space, page, err := s.findPage(ctx, instance, project, link)
	if err != nil {
		return nil, err
	}
	if page == nil || page.AgentAccess == constants.WikiAgentAccessHidden {
		return nil, errs.ErrNotFound
	}

	view := &model.WikiAgentPage{
		Slug:          agentSlugIn(space.Kind, page.Slug),
		Space:         space.Kind,
		Title:         page.Title,
		Summary:       page.Summary,
		AgentAccess:   page.AgentAccess,
		Version:       page.VersionNo,
		LatestVersion: page.VersionNo,
		Body:          page.Body,
	}
	if versionNo > 0 && versionNo != page.VersionNo {
		version, err := s.wiki.versionRepo.LoadOne(ctx, page.IdPage, versionNo)
		if err != nil {
			return nil, err
		}
		if version == nil {
			return nil, errs.ErrNotFound
		}
		view.Title, view.Summary, view.Body, view.Version = version.Title, version.Summary, version.Body, version.VersionNo
	}

	resolved, err := s.wiki.resolveLinks(ctx, instance, project, space, view.Body, true)
	if err != nil {
		return nil, err
	}
	view.Links = make([]model.WikiAgentLink, len(resolved))
	for i, link := range resolved {
		view.Links[i] = model.WikiAgentLink{
			Slug:   agentSlugIn(spaceKindOf(link.IdSpace, instance), link.Slug),
			Title:  link.Title,
			Exists: link.IdPage != nil,
		}
	}
	issues, err := s.wiki.loadPageIssues(ctx, user, page.IdPage, wikiListPageSize, 0)
	if err != nil {
		return nil, err
	}
	view.Issues = *issues

	if ref := s.runRef(ctx, user, idRun, idProject); ref != nil {
		s.recordQuietly(ctx, []*model.AgentRunWikiRead{
			ref.read(uuid.New(), constants.WikiReadMcpGet, page.IdPage, space.Kind, page.Slug, view.Title, view.Version,
				wikitext.EstimateTokens(view.Title+view.Body)),
		})
	}
	return view, nil
}

func (s *WikiAgentService) LoadPages(ctx context.Context, user model.User, idProject int64) ([]model.WikiAgentTreeNode, error) {
	instance, project, err := s.wiki.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	nodes, err := s.wiki.pageRepo.LoadTree(ctx, []int64{instance.IdSpace, project.IdSpace}, true)
	if err != nil {
		return nil, err
	}
	slugById := make(map[int64]string, len(nodes))
	for _, node := range nodes {
		slugById[node.IdPage] = agentSlugIn(spaceKindOf(node.IdSpace, instance), node.Slug)
	}
	result := make([]model.WikiAgentTreeNode, len(nodes))
	for i, node := range nodes {
		entry := model.WikiAgentTreeNode{
			Slug:        slugById[node.IdPage],
			Space:       spaceKindOf(node.IdSpace, instance),
			Title:       node.Title,
			AgentAccess: node.AgentAccess,
		}
		if node.IdParent != nil {
			if parent, ok := slugById[*node.IdParent]; ok {
				entry.Parent = &parent
			}
		}
		result[i] = entry
	}
	return result, nil
}

func (s *WikiAgentService) Upsert(ctx context.Context, user model.User, idProject int64, req model.WikiAgentUpsertReq) (*model.WikiAgentUpsertRes, error) {
	if user.IsAgent {
		return nil, errs.ErrForbidden.WithMessage("agents cannot write to the wiki")
	}
	target := req.Slug
	if target == "" {
		target = req.Title
	}
	link, ok := wikitext.ParseTarget(target)
	if !ok {
		return nil, errs.ErrWikiInvalidTitle
	}
	instance, project, err := s.wiki.projectSpaces(ctx, user, idProject)
	if err != nil {
		return nil, err
	}
	kind := constants.WikiSpaceProject
	if link.Shared || req.Space == constants.WikiSpaceInstance {
		kind = constants.WikiSpaceInstance
	}
	space := s.wiki.spaceByKind(instance, project, kind)
	page, err := s.wiki.pageRepo.LoadLivePageBySlug(ctx, space.IdSpace, link.Slug)
	if err != nil {
		return nil, err
	}
	if page != nil {
		return s.updatePage(ctx, user, kind, page, req)
	}
	return s.createPage(ctx, user, idProject, space, link.Slug, req)
}

func (s *WikiAgentService) updatePage(ctx context.Context, user model.User, kind constants.WikiSpaceKind, page *model.WikiPage, req model.WikiAgentUpsertReq) (*model.WikiAgentUpsertRes, error) {
	if page.AgentAccess == constants.WikiAgentAccessHidden {
		return nil, errs.ErrNotFound
	}
	if req.BaseVersion == 0 {
		return nil, errs.ErrConflict.WithMessage(fmt.Sprintf(
			"page %q already exists at version %d; read it with get_wiki_page and pass base_version", page.Slug, page.VersionNo))
	}
	summary := page.Summary
	if req.Summary != nil {
		summary = *req.Summary
	}
	agentAccess := page.AgentAccess
	if req.AgentAccess != "" {
		agentAccess = req.AgentAccess
	}
	result, err := s.wiki.Save(ctx, user, page.IdPage, model.SaveWikiPageReq{
		BaseVersion: req.BaseVersion,
		Title:       req.Title,
		Summary:     summary,
		Body:        req.Body,
		AgentAccess: agentAccess,
		Note:        req.Note,
	})
	if err != nil {
		return nil, err
	}
	if result.Conflict != nil {
		return nil, errs.ErrConflict.WithMessage(fmt.Sprintf(
			"page %q changed since version %d and the edits overlap; read version %d with get_wiki_page and apply the change again",
			page.Slug, req.BaseVersion, result.Conflict.Current.VersionNo))
	}
	return &model.WikiAgentUpsertRes{
		Slug:       agentSlugIn(kind, result.Page.Slug),
		Version:    result.Page.VersionNo,
		MergedFrom: result.MergedFrom,
	}, nil
}

func (s *WikiAgentService) createPage(ctx context.Context, user model.User, idProject int64, space *model.WikiSpace, slug string, req model.WikiAgentUpsertReq) (*model.WikiAgentUpsertRes, error) {
	if req.BaseVersion > 0 {
		return nil, errs.ErrNotFound.WithMessage(fmt.Sprintf("page %q does not exist anymore", slug))
	}
	if wikitext.Slugify(req.Title) != slug {
		return nil, errs.ErrBadRequest.WithMessage(fmt.Sprintf(
			"a new page gets its slug from the title; title %q gives %q, not %q", req.Title, wikitext.Slugify(req.Title), slug))
	}
	idParent, err := s.parentId(ctx, space, req.Parent)
	if err != nil {
		return nil, err
	}
	summary := ""
	if req.Summary != nil {
		summary = *req.Summary
	}
	created, err := s.wiki.Create(ctx, user, idProject, model.CreateWikiPageReq{
		Space:       space.Kind,
		IdParent:    idParent,
		Title:       req.Title,
		Summary:     summary,
		Body:        req.Body,
		AgentAccess: req.AgentAccess,
		Note:        req.Note,
	})
	if err != nil {
		return nil, err
	}
	return &model.WikiAgentUpsertRes{Slug: agentSlugIn(space.Kind, created.Slug), Version: created.VersionNo, Created: true}, nil
}

func (s *WikiAgentService) parentId(ctx context.Context, space *model.WikiSpace, parent *string) (*int64, error) {
	if parent == nil || *parent == "" {
		return nil, nil
	}
	link, ok := wikitext.ParseTarget(*parent)
	if !ok || link.Shared != (space.Kind == constants.WikiSpaceInstance) {
		return nil, errs.ErrWikiInvalidParent
	}
	page, err := s.wiki.pageRepo.LoadLivePageBySlug(ctx, space.IdSpace, link.Slug)
	if err != nil {
		return nil, err
	}
	if page == nil || page.AgentAccess == constants.WikiAgentAccessHidden {
		return nil, errs.ErrWikiInvalidParent
	}
	return &page.IdPage, nil
}

func (s *WikiAgentService) findPage(ctx context.Context, instance, project *model.WikiSpace, link wikitext.Link) (*model.WikiSpace, *model.WikiPage, error) {
	spaces := []*model.WikiSpace{project, instance}
	if link.Shared {
		spaces = []*model.WikiSpace{instance}
	}
	for _, space := range spaces {
		page, err := s.wiki.pageRepo.LoadLivePageBySlug(ctx, space.IdSpace, link.Slug)
		if err != nil {
			return nil, nil, err
		}
		if page != nil && page.AgentAccess != constants.WikiAgentAccessHidden {
			return space, page, nil
		}
	}
	return nil, nil, nil
}

func (s *WikiAgentService) runRef(ctx context.Context, user model.User, idRun, idProject int64) *wikiRunRef {
	if idRun == 0 {
		return nil
	}
	run, err := s.runRepo.LoadById(ctx, idRun)
	if err != nil {
		return nil
	}
	if run.IdUserAgent != user.IdUser || run.IdProject != idProject || run.FinishedAt != nil {
		return nil
	}
	tasks, err := s.taskRepo.LoadByRun(ctx, idRun)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Int64("idRun", idRun).Msg("wiki: loading run tasks for a read")
		return nil
	}
	var latest, active *model.AgentTask
	for _, task := range tasks {
		if latest == nil || task.IdTask > latest.IdTask {
			latest = task
		}
		if task.Status == constants.TaskStatusActive && (active == nil || task.IdTask > active.IdTask) {
			active = task
		}
	}
	if active != nil {
		latest = active
	}
	if latest == nil {
		return nil
	}
	return &wikiRunRef{idRun: idRun, idTask: latest.IdTask, stage: latest.Stage}
}

// A failed log write must never fail the read the agent asked for.
func (s *WikiAgentService) recordQuietly(ctx context.Context, reads []*model.AgentRunWikiRead) {
	if len(reads) == 0 {
		return
	}
	if err := s.readRepo.Insert(ctx, reads); err != nil {
		extctx.GetLogger(ctx).Error().Err(err).Int64("idRun", reads[0].IdRun).Msg("wiki: recording agent reads")
	}
}

func (ref *wikiRunRef) read(idCall uuid.UUID, source constants.WikiReadSource, idPage int64, kind constants.WikiSpaceKind, slug, title string, versionNo, tokens int) *model.AgentRunWikiRead {
	idTask := ref.idTask
	return &model.AgentRunWikiRead{
		IdRun:     ref.idRun,
		IdTask:    &idTask,
		Stage:     ref.stage,
		IdCall:    idCall,
		Source:    source,
		IdPage:    &idPage,
		SpaceKind: &kind,
		Slug:      &slug,
		Title:     &title,
		VersionNo: &versionNo,
		Tokens:    tokens,
	}
}

func spaceKindOf(idSpace int64, instance *model.WikiSpace) constants.WikiSpaceKind {
	if idSpace == instance.IdSpace {
		return constants.WikiSpaceInstance
	}
	return constants.WikiSpaceProject
}

func agentSlugIn(kind constants.WikiSpaceKind, slug string) string {
	if kind == constants.WikiSpaceInstance {
		return constants.WikiSharedSlugPrefix + slug
	}
	return slug
}

func clampSearchLimit(limit int) int {
	if limit <= 0 {
		return wikiAgentSearchDefaultLimit
	}
	return min(limit, wikiAgentSearchMaxLimit)
}
