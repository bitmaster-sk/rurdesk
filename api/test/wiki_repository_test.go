package test

import (
	"context"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/wikitext"
	"github.com/stretchr/testify/require"
)

func TestWikiSpaces_InstanceExistsAndProjectSpaceIsCreatedOnce(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "wiki-space")
	repo := injector.GetWikiSpaceRepository()
	ctx := context.Background()

	instance, err := repo.LoadInstanceSpace(ctx)
	require.Nil(t, err)
	require.Equal(t, constants.WikiSpaceInstance, instance.Kind)
	require.Nil(t, instance.IdProject)

	first, err := repo.EnsureProjectSpace(ctx, idProject)
	require.Nil(t, err)
	second, err := repo.EnsureProjectSpace(ctx, idProject)
	require.Nil(t, err)
	require.Equal(t, first.IdSpace, second.IdSpace)
	require.Equal(t, idProject, *second.IdProject)
}

func TestWikiPages_SlugIsUniqueAmongLivePagesOfOneSpace(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	ctx := context.Background()
	spaceA, err := injector.GetWikiSpaceRepository().EnsureProjectSpace(ctx, createProject(t, app, token, "wiki-slug-a"))
	require.Nil(t, err)
	spaceB, err := injector.GetWikiSpaceRepository().EnsureProjectSpace(ctx, createProject(t, app, token, "wiki-slug-b"))
	require.Nil(t, err)
	repo := injector.GetWikiPageRepository()

	original := insertWikiPage(t, spaceA.IdSpace, nil, "conventions", "Conventions", "", idUser)

	_, err = repo.InsertPage(ctx, &model.WikiPage{
		IdSpace: spaceA.IdSpace, Slug: "conventions", Title: "Again",
		AgentAccess: constants.WikiAgentAccessOnDemand, Rank: "n",
	}, idUser)
	require.ErrorIs(t, err, errs.ErrWikiSlugTaken, "a second live page with the same slug in one space must be rejected as taken")

	insertWikiPage(t, spaceB.IdSpace, nil, "conventions", "Conventions", "", idUser)

	require.Nil(t, repo.Trash(ctx, original.IdPage, []int64{original.IdPage}, idUser))
	insertWikiPage(t, spaceA.IdSpace, nil, "conventions", "Conventions v2", "", idUser)
}

func TestWikiPages_VersionsOnlyAccumulate(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	ctx := context.Background()
	space, err := injector.GetWikiSpaceRepository().EnsureProjectSpace(ctx, createProject(t, app, token, "wiki-versions"))
	require.Nil(t, err)
	repo := injector.GetWikiPageVersionRepository()
	page := insertWikiPage(t, space.IdSpace, nil, "history", "History", "one", idUser)

	for no, body := range []string{"one", "two", "three"} {
		require.Nil(t, repo.Insert(ctx, &model.WikiPageVersion{
			IdPage: page.IdPage, VersionNo: no + 1, Title: "History", Body: body,
			AgentAccess: constants.WikiAgentAccessOnDemand, CreateBy: &idUser,
		}))
	}
	require.NotNil(t, repo.Insert(ctx, &model.WikiPageVersion{
		IdPage: page.IdPage, VersionNo: 2, Title: "History", Body: "rewrite",
		AgentAccess: constants.WikiAgentAccessOnDemand,
	}), "an existing version number must never be written twice")

	versions, err := repo.Load(ctx, page.IdPage)
	require.Nil(t, err)
	require.Len(t, versions, 3)
	require.Equal(t, 3, versions[0].VersionNo)

	second, err := repo.LoadOne(ctx, page.IdPage, 2)
	require.Nil(t, err)
	require.Equal(t, "two", second.Body)
}

func TestWikiPages_TrashMovesSubtreeAndRestoreBringsItBack(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	ctx := context.Background()
	space, err := injector.GetWikiSpaceRepository().EnsureProjectSpace(ctx, createProject(t, app, token, "wiki-trash"))
	require.Nil(t, err)
	repo := injector.GetWikiPageRepository()

	root := insertWikiPage(t, space.IdSpace, nil, "ops", "Ops", "", idUser)
	child := insertWikiPage(t, space.IdSpace, &root.IdPage, "deploy", "Deploy", "", idUser)
	insertWikiPage(t, space.IdSpace, &child.IdPage, "rollback", "Rollback", "", idUser)
	sibling := insertWikiPage(t, space.IdSpace, nil, "glossary", "Glossary", "", idUser)

	ids, err := repo.LoadLiveSubtreeIds(ctx, root.IdPage)
	require.Nil(t, err)
	require.Len(t, ids, 3)
	require.Nil(t, repo.Trash(ctx, root.IdPage, ids, idUser))

	tree, err := repo.LoadTree(ctx, []int64{space.IdSpace}, false)
	require.Nil(t, err)
	require.Len(t, tree, 1)
	require.Equal(t, sibling.IdPage, tree[0].IdPage)

	trash, err := repo.LoadTrash(ctx, []int64{space.IdSpace}, false)
	require.Nil(t, err)
	require.Len(t, trash, 1)
	require.Equal(t, root.IdPage, trash[0].IdPage)
	require.Equal(t, 2, trash[0].Descendants)
	require.True(t, trash[0].ParentAlive)

	require.Nil(t, repo.Restore(ctx, root.IdPage))
	tree, err = repo.LoadTree(ctx, []int64{space.IdSpace}, false)
	require.Nil(t, err)
	require.Len(t, tree, 4)
}

func TestWikiPages_SearchRanksTitleAboveBodyAndReturnsSnippet(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	ctx := context.Background()
	space, err := injector.GetWikiSpaceRepository().EnsureProjectSpace(ctx, createProject(t, app, token, "wiki-search"))
	require.Nil(t, err)

	insertWikiPage(t, space.IdSpace, nil, "notes", "Notes", "the worktree retention runs nightly", idUser)
	titled := insertWikiPage(t, space.IdSpace, nil, "worktree", "Worktree retention", "how long we keep it", idUser)
	insertWikiPage(t, space.IdSpace, nil, "unrelated", "Unrelated", "nothing to see", idUser)

	hits, err := injector.GetWikiPageRepository().Search(ctx, []int64{space.IdSpace}, wikitext.ToPrefixQuery("worktree retention"), 10, false)
	require.Nil(t, err)
	require.Len(t, hits, 2)
	require.Equal(t, titled.IdPage, hits[0].IdPage)
	require.Contains(t, hits[1].Snippet, "<<worktree>>")

	partial, err := injector.GetWikiPageRepository().Search(ctx, []int64{space.IdSpace}, wikitext.ToPrefixQuery("worktr reten"), 10, false)
	require.Nil(t, err)
	require.Len(t, partial, 2, "the start of a word is enough")

	none, err := injector.GetWikiPageRepository().Search(ctx, []int64{space.IdSpace}, wikitext.ToPrefixQuery("worktree nightlyx"), 10, false)
	require.Nil(t, err)
	require.Empty(t, none)
}

func TestWikiIssuePages_DescriptionSyncKeepsManualLinks(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	ctx := context.Background()
	idProject := createProject(t, app, token, "wiki-issue-links")
	space, err := injector.GetWikiSpaceRepository().EnsureProjectSpace(ctx, idProject)
	require.Nil(t, err)
	iss := createIssue(t, app, token, idProject, "linked")
	repo := injector.GetWikiIssuePageRepository()

	described := insertWikiPage(t, space.IdSpace, nil, "described", "Described", "", idUser)
	manual := insertWikiPage(t, space.IdSpace, nil, "manual", "Manual", "", idUser)

	require.Nil(t, repo.Insert(ctx, iss.IdIssue, manual.IdPage, constants.WikiIssueLinkManual))
	require.Nil(t, repo.ReplaceFromDescription(ctx, iss.IdIssue, []int64{described.IdPage}))

	pages, err := repo.LoadByIssue(ctx, iss.IdIssue, false)
	require.Nil(t, err)
	require.Len(t, pages, 2)

	require.Nil(t, repo.ReplaceFromDescription(ctx, iss.IdIssue, []int64{}))
	pages, err = repo.LoadByIssue(ctx, iss.IdIssue, false)
	require.Nil(t, err)
	require.Len(t, pages, 1)
	require.Equal(t, manual.IdPage, pages[0].IdPage)
	require.Equal(t, constants.WikiIssueLinkManual, pages[0].Source)
}

func TestWikiSpaces_BudgetsCoverAllProjectsWhenNoFilterIsGiven(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	first := createProject(t, app, token, "wiki-budget-all-a")
	second := createProject(t, app, token, "wiki-budget-all-b")
	repo := injector.GetWikiSpaceRepository()
	ctx := context.Background()
	_, err := repo.EnsureProjectSpace(ctx, first)
	require.Nil(t, err)

	all, err := repo.LoadProjectBudgets(ctx, nil)
	require.Nil(t, err)
	byProject := map[int64]*repository.WikiProjectBudget{}
	for _, budget := range all {
		byProject[budget.IdProject] = budget
	}
	require.NotNil(t, byProject[first])
	require.NotNil(t, byProject[first].IdSpace)
	require.NotNil(t, byProject[second])
	require.Nil(t, byProject[second].IdSpace)
	require.Equal(t, 12000, byProject[second].Limit)

	one, err := repo.LoadProjectBudgets(ctx, []int64{second})
	require.Nil(t, err)
	require.Len(t, one, 1)
}

func TestWikiLocks_LockedPageDoesNotBlockAChildInsertHoldingTheBudgetLock(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	ctx := context.Background()
	space, err := injector.GetWikiSpaceRepository().EnsureProjectSpace(ctx, createProject(t, app, token, "wiki-locks"))
	require.Nil(t, err)
	parent := insertWikiPage(t, space.IdSpace, nil, "locked-parent", "Locked parent", "", idUser)

	saving, err := app.Pool.Begin(ctx)
	require.Nil(t, err)
	defer func() { _ = saving.Rollback(ctx) }()
	_, err = injector.GetWikiPageRepository().LockPage(extctx.WithTx(ctx, saving), parent.IdPage)
	require.Nil(t, err)

	creating, err := app.Pool.Begin(ctx)
	require.Nil(t, err)
	defer func() { _ = creating.Rollback(ctx) }()
	_, err = creating.Exec(ctx, "SET LOCAL lock_timeout = '2s'")
	require.Nil(t, err)
	creatingCtx := extctx.WithTx(ctx, creating)
	require.Nil(t, injector.GetWikiSpaceRepository().LockBudget(creatingCtx))
	_, err = injector.GetWikiPageRepository().InsertPage(creatingCtx, &model.WikiPage{
		IdSpace: space.IdSpace, IdParent: &parent.IdPage, Slug: "locked-child", Title: "Locked child",
		AgentAccess: constants.WikiAgentAccessAlways, Rank: "n",
	}, idUser)
	require.Nil(t, err, "inserting a child must not wait for the lock on its parent")
	require.Nil(t, creating.Commit(ctx))
}

func TestWikiPages_SubtreeQueriesStopOnAParentCycle(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idUser := idOfUser(t, app, token, "test@test.sk")
	ctx := context.Background()
	space, err := injector.GetWikiSpaceRepository().EnsureProjectSpace(ctx, createProject(t, app, token, "wiki-cycle"))
	require.Nil(t, err)
	first := insertWikiPage(t, space.IdSpace, nil, "cycle-a", "Cycle A", "", idUser)
	second := insertWikiPage(t, space.IdSpace, &first.IdPage, "cycle-b", "Cycle B", "", idUser)
	_, err = app.Pool.Exec(ctx, "UPDATE wiki.page SET id_parent = $1 WHERE id_page = $2", second.IdPage, first.IdPage)
	require.Nil(t, err)

	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	subtree, err := injector.GetWikiPageRepository().LoadLiveSubtreeIds(bounded, first.IdPage)
	require.Nil(t, err)
	require.ElementsMatch(t, []int64{first.IdPage, second.IdPage}, subtree)
	ancestors, err := injector.GetWikiPageRepository().LoadLiveAncestorIds(bounded, first.IdPage)
	require.Nil(t, err)
	require.ElementsMatch(t, []int64{first.IdPage, second.IdPage}, ancestors)
}

func insertWikiPage(t *testing.T, idSpace int64, idParent *int64, slug, title, body string, idUser int64) *model.WikiPage {
	page, err := injector.GetWikiPageRepository().InsertPage(context.Background(), &model.WikiPage{
		IdSpace:     idSpace,
		IdParent:    idParent,
		Slug:        slug,
		Title:       title,
		Body:        body,
		AgentAccess: constants.WikiAgentAccessOnDemand,
		Rank:        "m",
	}, idUser)
	require.Nil(t, err)
	return page
}
