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

type WikiProposalController struct {
	proposals *service.WikiProposalService
}

func NewWikiProposalController(proposals *service.WikiProposalService) *WikiProposalController {
	return &WikiProposalController{proposals: proposals}
}

func (pc *WikiProposalController) Suggest(c *gin.Context) {
	idProject, ok := pc.idParam(c, "idProject")
	if !ok {
		return
	}
	var req model.WikiProposalSuggestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	idRun := req.IdRun
	if header, err := strconv.ParseInt(c.GetHeader(constants.AgentRunHeader), 10, 64); err == nil {
		idRun = header
	}
	view, err := pc.proposals.Suggest(c.Request.Context(), pc.user(c), idProject, idRun, req)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, view)
}

func (pc *WikiProposalController) GetRunProposals(c *gin.Context) {
	idProject, ok := pc.idParam(c, "idProject")
	if !ok {
		return
	}
	idRun, err := strconv.ParseInt(c.Query("run"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage("run must be the id of the agent run"))
		return
	}
	views, err := pc.proposals.LoadForAgentRun(c.Request.Context(), pc.user(c), idProject, idRun)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, views)
}

func (pc *WikiProposalController) GetIssueProposals(c *gin.Context) {
	idIssue, ok := pc.idParam(c, "idIssue")
	if !ok {
		return
	}
	proposals, err := pc.proposals.LoadByIssue(c.Request.Context(), pc.user(c), idIssue)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, proposals)
}

func (pc *WikiProposalController) GetOpenProposals(c *gin.Context) {
	idProject, ok := pc.idParam(c, "idProject")
	if !ok {
		return
	}
	proposals, err := pc.proposals.LoadOpen(c.Request.Context(), pc.user(c), idProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, proposals)
}

func (pc *WikiProposalController) GetProposal(c *gin.Context) {
	idProposal, ok := pc.idParam(c, "idProposal")
	if !ok {
		return
	}
	detail, err := pc.proposals.LoadDetail(c.Request.Context(), pc.user(c), idProposal)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, detail)
}

func (pc *WikiProposalController) Accept(c *gin.Context) {
	idProposal, ok := pc.idParam(c, "idProposal")
	if !ok {
		return
	}
	var req model.WikiProposalAcceptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	result, err := pc.proposals.Accept(c.Request.Context(), pc.user(c), idProposal, req)
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

func (pc *WikiProposalController) Reject(c *gin.Context) {
	idProposal, ok := pc.idParam(c, "idProposal")
	if !ok {
		return
	}
	var req model.WikiProposalRejectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	proposal, err := pc.proposals.Reject(c.Request.Context(), pc.user(c), idProposal, req)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, proposal)
}

func (pc *WikiProposalController) idParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return 0, false
	}
	return id, true
}

func (pc *WikiProposalController) user(c *gin.Context) model.User {
	user, _ := extctx.GetUser(c.Request.Context())
	return user
}
