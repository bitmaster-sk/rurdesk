package controller

import (
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type WikiController struct {
	wiki *service.WikiService
}

func NewWikiController(wiki *service.WikiService) *WikiController {
	return &WikiController{wiki: wiki}
}

func (wc *WikiController) GetTree(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	tree, err := wc.wiki.LoadTree(c.Request.Context(), wc.user(c), idProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, tree)
}

func (wc *WikiController) GetPage(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	kind := constants.WikiSpaceKind(c.Param("space"))
	if kind != constants.WikiSpaceInstance && kind != constants.WikiSpaceProject {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}
	view, err := wc.wiki.LoadPage(c.Request.Context(), wc.user(c), idProject, kind, c.Param("slug"))
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, view)
}

func (wc *WikiController) CreatePage(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	var req model.CreateWikiPageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	page, err := wc.wiki.Create(c.Request.Context(), wc.user(c), idProject, req)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, page)
}

func (wc *WikiController) SavePage(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	var req model.SaveWikiPageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	result, err := wc.wiki.Save(c.Request.Context(), wc.user(c), idPage, req)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if result.Conflict != nil {
		c.JSON(http.StatusConflict, result.Conflict)
		return
	}
	ResponseOk(c, result)
}

func (wc *WikiController) MergePreview(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	var req model.WikiMergePreviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	preview, err := wc.wiki.MergePreview(c.Request.Context(), wc.user(c), idPage, req)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, preview)
}

func (wc *WikiController) MovePage(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	var req model.MoveWikiPageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	if err := wc.wiki.Move(c.Request.Context(), wc.user(c), idPage, req); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) TrashPage(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	if err := wc.wiki.Trash(c.Request.Context(), wc.user(c), idPage); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) GetTrash(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	items, err := wc.wiki.LoadTrash(c.Request.Context(), wc.user(c), idProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, items)
}

func (wc *WikiController) RestorePage(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	page, err := wc.wiki.Restore(c.Request.Context(), wc.user(c), idPage)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, page)
}

func (wc *WikiController) PurgePage(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	if err := wc.wiki.Purge(c.Request.Context(), wc.user(c), idPage); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) GetVersions(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	versions, err := wc.wiki.LoadVersions(c.Request.Context(), wc.user(c), idPage)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, versions)
}

func (wc *WikiController) GetVersion(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	versionNo, err := strconv.Atoi(c.Param("versionNo"))
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}
	version, err := wc.wiki.LoadVersion(c.Request.Context(), wc.user(c), idPage, versionNo)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, version)
}

func (wc *WikiController) GetDiff(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	from, ok := wc.intQuery(c, "from")
	if !ok {
		return
	}
	to, ok := wc.intQuery(c, "to")
	if !ok {
		return
	}
	diff, err := wc.wiki.Diff(c.Request.Context(), wc.user(c), idPage, from, to)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, gin.H{"diff": diff})
}

func (wc *WikiController) RevertPage(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	versionNo, err := strconv.Atoi(c.Param("versionNo"))
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}
	page, err := wc.wiki.Revert(c.Request.Context(), wc.user(c), idPage, versionNo)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, page)
}

func (wc *WikiController) GetDraft(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	draft, err := wc.wiki.LoadDraft(c.Request.Context(), wc.user(c), idPage)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, draft)
}

func (wc *WikiController) SaveDraft(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	var req model.WikiDraftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	if err := wc.wiki.SaveDraft(c.Request.Context(), wc.user(c), idPage, req); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) DeleteDraft(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	if err := wc.wiki.DeleteDraft(c.Request.Context(), wc.user(c), idPage); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) Heartbeat(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	isEditing := c.Request.Method != http.MethodDelete
	editors, err := wc.wiki.Heartbeat(c.Request.Context(), wc.user(c), idPage, isEditing)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, editors)
}

func (wc *WikiController) Search(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	hits, err := wc.wiki.Search(c.Request.Context(), wc.user(c), idProject, c.Query("q"))
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, hits)
}

func (wc *WikiController) UpdateHome(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	var req model.WikiHomeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	if err := wc.wiki.UpdateHomePage(c.Request.Context(), wc.user(c), idProject, req); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) UpdateSettings(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	var req model.WikiSettingsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	if err := wc.wiki.UpdateSettings(c.Request.Context(), wc.user(c), idProject, req); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) GetIssueLinks(c *gin.Context) {
	idIssue, ok := wc.idParam(c, "idIssue")
	if !ok {
		return
	}
	links, err := wc.wiki.LoadIssueLinks(c.Request.Context(), wc.user(c), idIssue)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, links)
}

func (wc *WikiController) AddIssueLink(c *gin.Context) {
	idIssue, ok := wc.idParam(c, "idIssue")
	if !ok {
		return
	}
	var req model.WikiIssuePageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	if err := wc.wiki.AddIssueLink(c.Request.Context(), wc.user(c), idIssue, req.IdPage); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) RemoveIssueLink(c *gin.Context) {
	idIssue, ok := wc.idParam(c, "idIssue")
	if !ok {
		return
	}
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	if err := wc.wiki.RemoveIssueLink(c.Request.Context(), wc.user(c), idIssue, idPage); err != nil {
		ResponseErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (wc *WikiController) GetPageIssues(c *gin.Context) {
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	limit, offset, ok := wc.paging(c)
	if !ok {
		return
	}
	issues, err := wc.wiki.LoadPageIssues(c.Request.Context(), wc.user(c), idPage, limit, offset)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, issues)
}

func (wc *WikiController) GetPageBacklinks(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	idPage, ok := wc.idParam(c, "idPage")
	if !ok {
		return
	}
	limit, offset, ok := wc.paging(c)
	if !ok {
		return
	}
	backlinks, err := wc.wiki.LoadPageBacklinks(c.Request.Context(), wc.user(c), idProject, idPage, limit, offset)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, backlinks)
}

func (wc *WikiController) idParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return 0, false
	}
	return id, true
}

func (wc *WikiController) intQuery(c *gin.Context, name string) (int, bool) {
	value, err := strconv.Atoi(c.Query(name))
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return 0, false
	}
	return value, true
}

func (wc *WikiController) paging(c *gin.Context) (int, int, bool) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "0"))
	if err != nil || limit < 0 {
		ResponseErr(c, errs.ErrBadRequest.WithMessage("limit must be a non-negative number"))
		return 0, 0, false
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		ResponseErr(c, errs.ErrBadRequest.WithMessage("offset must be a non-negative number"))
		return 0, 0, false
	}
	return limit, offset, true
}

func (wc *WikiController) user(c *gin.Context) model.User {
	user, _ := extctx.GetUser(c.Request.Context())
	return user
}
