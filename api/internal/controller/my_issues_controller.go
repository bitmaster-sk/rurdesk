package controller

import (
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/gin-gonic/gin"
)

type MyIssuesController struct {
	issueRepo *repository.IssueRepository
}

func NewMyIssuesController(issueRepo *repository.IssueRepository) *MyIssuesController {
	return &MyIssuesController{issueRepo: issueRepo}
}

func (mic *MyIssuesController) GetMyIssues(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	filter := &model.LoadIssuesFilter{
		IdsAssignedTo:      []int64{user.IdUser},
		ExcludeFinalStates: true,
	}

	if idProjectStr := c.Query("idProject"); idProjectStr != "" {
		idProject, err := strconv.ParseInt(idProjectStr, 10, 64)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusBadRequest)
			return
		}
		filter.IdProject = idProject
	}

	limit := int64(50)
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.ParseInt(limitStr, 10, 64); err == nil && parsed > 0 {
			if parsed > 200 {
				parsed = 200
			}
			limit = parsed
		}
	}
	filter.Limit = &limit

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.ParseInt(offsetStr, 10, 64); err == nil && offset >= 0 {
			filter.Offset = &offset
		}
	}

	issues, err := mic.issueRepo.LoadIssues(ctx, filter)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, issues)
}
