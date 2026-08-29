package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

type PinController struct {
	pinRepo *repository.PinRepository
	acl     *service.AclService
}

func NewPinController(pr *repository.PinRepository, acl *service.AclService) *PinController {
	return &PinController{
		pinRepo: pr,
		acl:     acl,
	}
}

func (pc *PinController) GetPins(c *gin.Context) {
	idPinDestination, err := strconv.ParseInt(c.Query("idPinDestination"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}
	idPinDestinationType, err := strconv.Atoi(c.Query("idPinDestinationType"))
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	pdt, err := pc.pinRepo.LoadPinDestinationType(ctx, idPinDestinationType)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	var canRead bool
	switch pdt.Code {
	case "user-page":
		canRead = user.IdUser == idPinDestination
	case "project-page":
		canRead = pc.acl.CanReadProject(ctx, user.IdUser, idPinDestination)
	}
	if !canRead {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	pins, err := pc.pinRepo.LoadPinnedIssues(ctx, idPinDestination, idPinDestinationType)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// Drop pinned issues the viewer cannot read: issue visibility is project-scoped, so a
	// pin may reference an issue in an inaccessible project. Reading the
	// destination does not imply access to every pinned issue's project.
	visible := make([]*model.Pin, 0, len(pins))
	for _, pin := range pins {
		if pin.Issue != nil && pc.acl.CanReadProject(ctx, user.IdUser, pin.Issue.IdProject) {
			visible = append(visible, pin)
		}
	}

	if len(visible) == 0 {
		c.JSON(http.StatusOK, []model.Issue{})
		return
	}
	c.JSON(http.StatusOK, visible)
}

func (pc *PinController) CreatePin(c *gin.Context) {
	var dto model.CreatePinReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	pdt, err := pc.pinRepo.LoadPinDestinationType(ctx, dto.IdPinDestinationType)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	var canRead bool
	switch pdt.Code {
	case "user-page":
		canRead = user.IdUser == dto.IdPinDestination
	case "project-page":
		canRead = pc.acl.CanReadProject(ctx, user.IdUser, dto.IdPinDestination)
	}
	if !canRead {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	// The check above only covers where the pin goes. Without this one the issue
	// being pinned is never validated, so any member could pin issues out of
	// projects they cannot read — the read side hides them, but the write side
	// still confirms which issue ids exist.
	idIssueProject, err := pc.pinRepo.ProjectOfIssue(ctx, dto.IdIssue)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = c.Error(errForbidden)
			c.Status(http.StatusForbidden)
			return
		}
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	if !pc.acl.CanReadProject(ctx, user.IdUser, idIssueProject) {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	pin := &model.Pin{
		IdIssue:              dto.IdIssue,
		IdPinDestination:     dto.IdPinDestination,
		IdPinDestinationType: dto.IdPinDestinationType,
	}
	if err := pc.pinRepo.InsertPinnedIssue(ctx, pin); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (pc *PinController) DeletePin(c *gin.Context) {
	idPin, err := strconv.ParseInt(c.Param("idPin"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	pin, err := pc.pinRepo.LoadPinnedIssue(ctx, idPin)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	pdt, err := pc.pinRepo.LoadPinDestinationType(ctx, pin.IdPinDestinationType)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	var canDelete bool
	switch pdt.Code {
	case "user-page":
		canDelete = user.IdUser == pin.IdPinDestination
	case "project-page":
		canDelete = pc.acl.CanReadProject(ctx, user.IdUser, pin.IdPinDestination)
	}
	if !canDelete {
		_ = c.Error(errForbidden)
		c.Status(http.StatusForbidden)
		return
	}

	if err := pc.pinRepo.DeletePinnedIssue(ctx, pin.IdPin); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (pc *PinController) GetPinDestinationTypes(c *gin.Context) {
	ctx := c.Request.Context()
	pdts, err := pc.pinRepo.LoadPinDestinationTypes(ctx)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, pdts)
}
