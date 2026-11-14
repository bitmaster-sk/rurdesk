package scim

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/gin-gonic/gin"
)

const maxBodyBytes = 1 << 20

type Handler struct {
	service *Service
	license func() license.Provider
}

func NewHandler(service *Service, licenseProvider func() license.Provider) *Handler {
	return &Handler{service: service, license: licenseProvider}
}

func (h *Handler) Register(engine *gin.Engine) {
	group := engine.Group(BasePath, h.authenticate)
	group.GET("/ServiceProviderConfig", h.requireLicense, serviceProviderConfig)
	group.GET("/Schemas", h.requireLicense, schemas)
	group.GET("/ResourceTypes", h.requireLicense, resourceTypes)
	group.GET("/Users", h.requireLicense, h.List)
	group.GET("/Users/:id", h.requireLicense, h.Get)
	group.POST("/Users", h.requireLicense, h.Create)
	group.PUT("/Users/:id", h.Replace)
	group.PATCH("/Users/:id", h.Patch)
	group.DELETE("/Users/:id", h.Delete)
}

func (h *Handler) authenticate(c *gin.Context) {
	err := h.service.Authenticate(c.Request.Context(), extctx.BearerToken(c.GetHeader("Authorization")))
	if err != nil {
		respondError(c, err)
		c.Abort()
	}
}

func (h *Handler) isLicensed(c *gin.Context) bool {
	return h.license().IsAvailable(c.Request.Context(), license.SSO)
}

func (h *Handler) requireLicense(c *gin.Context) {
	if !h.isLicensed(c) {
		respondError(c, errForbidden)
		c.Abort()
	}
}

func (h *Handler) List(c *gin.Context) {
	filter, err := ParseFilter(c.Query("filter"))
	if err != nil {
		respondError(c, err)
		return
	}
	list, err := h.service.List(c.Request.Context(), filter, ParsePage(c.Query("startIndex"), c.Query("count")))
	if err != nil {
		respondError(c, err)
		return
	}
	respond(c, http.StatusOK, list)
}

func (h *Handler) Get(c *gin.Context) {
	idUser, ok := userId(c)
	if !ok {
		return
	}
	resource, err := h.service.Get(c.Request.Context(), idUser)
	if err != nil {
		respondError(c, err)
		return
	}
	respond(c, http.StatusOK, resource)
}

func (h *Handler) Create(c *gin.Context) {
	changes, ok := readChanges(c, ParseResource)
	if !ok {
		return
	}
	resource, err := h.service.Create(c.Request.Context(), changes)
	if err != nil {
		respondError(c, err)
		return
	}
	respond(c, http.StatusCreated, resource)
}

func (h *Handler) Replace(c *gin.Context) {
	h.update(c, ParseResource)
}

func (h *Handler) Patch(c *gin.Context) {
	h.update(c, ParsePatch)
}

func (h *Handler) update(c *gin.Context, parse func([]byte) (Changes, error)) {
	idUser, ok := userId(c)
	if !ok {
		return
	}
	changes, ok := readChanges(c, parse)
	if !ok {
		return
	}
	resource, err := h.service.Update(c.Request.Context(), idUser, changes, h.isLicensed(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respond(c, http.StatusOK, resource)
}

func (h *Handler) Delete(c *gin.Context) {
	idUser, ok := userId(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), idUser); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func readChanges(c *gin.Context, parse func([]byte) (Changes, error)) (Changes, bool) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBodyBytes))
	if err != nil {
		respondError(c, invalidSyntax("the body cannot be read"))
		return Changes{}, false
	}
	changes, err := parse(body)
	if err != nil {
		respondError(c, err)
		return Changes{}, false
	}
	return changes, true
}

func userId(c *gin.Context) (int64, bool) {
	idUser, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, errNotFound)
		return 0, false
	}
	return idUser, true
}

func respond(c *gin.Context, status int, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Data(status, ContentType, encoded)
}

func respondError(c *gin.Context, err error) {
	var scimErr *Error
	if !errors.As(err, &scimErr) {
		extctx.GetLogger(c.Request.Context()).Error().Err(err).Msg("scim: request failed")
		scimErr = errInternal
	}
	encoded, _ := json.Marshal(scimErr.Body())
	c.Data(scimErr.Status, ContentType, encoded)
}
