package controller

import (
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type WikiAgentController struct {
	wikiAgent *service.WikiAgentService
}

func NewWikiAgentController(wikiAgent *service.WikiAgentService) *WikiAgentController {
	return &WikiAgentController{wikiAgent: wikiAgent}
}

func (wc *WikiAgentController) Search(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	limit, ok := wc.optionalInt(c, "limit")
	if !ok {
		return
	}
	hits, err := wc.wikiAgent.Search(c.Request.Context(), wc.user(c), idProject, c.Query("query"), limit, wc.idRun(c))
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, hits)
}

func (wc *WikiAgentController) GetPage(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	version, ok := wc.optionalInt(c, "version")
	if !ok {
		return
	}
	page, err := wc.wikiAgent.LoadPage(c.Request.Context(), wc.user(c), idProject, c.Param("slug"), version, wc.idRun(c))
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, page)
}

func (wc *WikiAgentController) GetPages(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	pages, err := wc.wikiAgent.LoadPages(c.Request.Context(), wc.user(c), idProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, pages)
}

func (wc *WikiAgentController) UpsertPage(c *gin.Context) {
	idProject, ok := wc.idParam(c, "idProject")
	if !ok {
		return
	}
	var req model.WikiAgentUpsertReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	result, err := wc.wikiAgent.Upsert(c.Request.Context(), wc.user(c), idProject, req)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, result)
}

func (wc *WikiAgentController) GetRunReads(c *gin.Context) {
	idRun, ok := wc.idParam(c, "idRun")
	if !ok {
		return
	}
	reads, err := wc.wikiAgent.LoadRunReads(c.Request.Context(), wc.user(c), idRun)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, reads)
}

func (wc *WikiAgentController) idParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return 0, false
	}
	return id, true
}

func (wc *WikiAgentController) optionalInt(c *gin.Context, name string) (int, bool) {
	raw := c.Query(name)
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(name+" must be a non-negative number"))
		return 0, false
	}
	return value, true
}

// An unparsable run header only means the read is not logged, so it is ignored rather than rejected.
func (wc *WikiAgentController) idRun(c *gin.Context) int64 {
	idRun, err := strconv.ParseInt(c.GetHeader(constants.AgentRunHeader), 10, 64)
	if err != nil {
		return 0
	}
	return idRun
}

func (wc *WikiAgentController) user(c *gin.Context) model.User {
	user, _ := extctx.GetUser(c.Request.Context())
	return user
}
