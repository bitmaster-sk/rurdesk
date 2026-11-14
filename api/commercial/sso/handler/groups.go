package handler

import (
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/groups"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/links"
	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/gin-gonic/gin"
)

func (h *Handler) ListMappings(c *gin.Context) {
	mappings, err := h.deps.Groups.ListMappings(c.Request.Context())
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	controller.ResponseOk(c, mappings)
}

func (h *Handler) InsertMapping(c *gin.Context) {
	var req groups.InsertMappingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		controller.ResponseErr(c, errs.ErrValidation)
		return
	}
	if err := h.deps.Groups.InsertMapping(c.Request.Context(), req); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.ListMappings(c)
}

func (h *Handler) DeleteMapping(c *gin.Context) {
	idMapping, ok := pathId(c, "idMapping")
	if !ok {
		return
	}
	if err := h.deps.Groups.DeleteMapping(c.Request.Context(), idMapping); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.ListMappings(c)
}

func (h *Handler) ListIdentities(c *gin.Context) {
	idUser, ok := pathId(c, "idUser")
	if !ok {
		return
	}
	identities, err := h.deps.Links.List(c.Request.Context(), idUser)
	if err != nil {
		controller.ResponseErr(c, err)
		return
	}
	controller.ResponseOk(c, identities)
}

func (h *Handler) UnlinkIdentity(c *gin.Context) {
	idUser, ok := pathId(c, "idUser")
	if !ok {
		return
	}
	if err := h.deps.Links.Unlink(c.Request.Context(), idUser); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.ListIdentities(c)
}

func (h *Handler) MoveIdentity(c *gin.Context) {
	idUser, ok := pathId(c, "idUser")
	if !ok {
		return
	}
	var req links.MoveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		controller.ResponseErr(c, errs.ErrValidation)
		return
	}
	ctx := c.Request.Context()
	acting, _ := extctx.GetUser(ctx)
	if err := h.deps.Links.Move(ctx, idUser, req.IdTargetUser, acting.IdUser); err != nil {
		controller.ResponseErr(c, err)
		return
	}
	h.ListIdentities(c)
}

func pathId(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		controller.ResponseErr(c, errs.ErrBadRequest)
		return 0, false
	}
	return id, true
}
