package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
)

type IssueHistoryController struct {
	issueRepo  *repository.IssueRepository
	historySvc *service.IssueHistoryService
	acl        *service.AclService
}

func NewIssueHistoryController(issueRepo *repository.IssueRepository, historySvc *service.IssueHistoryService, acl *service.AclService) *IssueHistoryController {
	return &IssueHistoryController{issueRepo: issueRepo, historySvc: historySvc, acl: acl}
}

func (ihc *IssueHistoryController) GetHistory(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idIssuePublic, err := strconv.ParseInt(c.Param("idIssuePublic"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if !ihc.acl.CanReadProject(ctx, user.IdUser, idProject) {
		_ = c.Error(errs.ErrForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	issue, err := ihc.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{
		IdProject:     &idProject,
		IdIssuePublic: &idIssuePublic,
	})
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusNotFound)
		return
	}

	history, users, err := ihc.historySvc.List(ctx, issue.IdIssue)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	res := model.ReadIssueHistoryRes{
		History: make([]model.ReadIssueHistoryDto, 0, len(history)),
		Users:   make([]model.IssueHistoryUserDto, 0, len(users)),
	}
	for _, row := range history {
		res.History = append(res.History, model.NewReadIssueHistoryDto(row))
	}
	for _, user := range users {
		res.Users = append(res.Users, model.NewIssueHistoryUserDto(user))
	}
	c.JSON(http.StatusOK, res)
}
