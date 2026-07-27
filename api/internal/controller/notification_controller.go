package controller

import (
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/gin-gonic/gin"
)

type NotificationController struct {
	notifRepo *repository.NotificationRepository
}

func NewNotificationController(notifRepo *repository.NotificationRepository) *NotificationController {
	return &NotificationController{notifRepo: notifRepo}
}

func (nc *NotificationController) List(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	filter := &model.NotificationListFilter{
		IdUser: user.IdUser,
		Limit:  50,
	}

	if idProjectStr := c.Query("idProject"); idProjectStr != "" {
		idProject, err := strconv.ParseInt(idProjectStr, 10, 64)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusBadRequest)
			return
		}
		filter.IdProject = &idProject
	}

	if c.Query("unread") == "true" {
		filter.OnlyUnread = true
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusBadRequest)
			return
		}
		filter.Limit = limit
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			_ = c.Error(err)
			c.Status(http.StatusBadRequest)
			return
		}
		filter.Offset = offset
	}

	notifications, err := nc.notifRepo.GetForUser(ctx, filter)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []*model.Notification{}
	}
	c.JSON(http.StatusOK, notifications)
}

func (nc *NotificationController) MarkAllRead(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	var dto model.MarkAllReadReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	if err := nc.notifRepo.MarkAllRead(ctx, user.IdUser, dto.IdProject); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (nc *NotificationController) MarkRead(c *gin.Context) {
	idNotification, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if err := nc.notifRepo.MarkRead(ctx, idNotification, user.IdUser); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (nc *NotificationController) Delete(c *gin.Context) {
	idNotification, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	if err := nc.notifRepo.Delete(ctx, idNotification, user.IdUser); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}
