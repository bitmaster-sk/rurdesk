package controller

import (
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type SkillController struct {
	skillService *service.SkillService
}

func NewSkillController(skillService *service.SkillService) *SkillController {
	return &SkillController{skillService: skillService}
}

func (ctrl *SkillController) respondSkillErr(c *gin.Context, err error) {
	var appErr *errs.Error
	if errs.As(err, &appErr) {
		_ = c.Error(appErr)
		c.Status(appErr.HttpStatus())
		return
	}
	_ = c.Error(err)
	c.Status(http.StatusInternalServerError)
}

func (ctrl *SkillController) List(c *gin.Context) {
	all, err := ctrl.skillService.Load(c.Request.Context())
	if err != nil {
		ctrl.respondSkillErr(c, err)
		return
	}
	if all == nil {
		all = []*model.Skill{}
	}
	c.JSON(http.StatusOK, all)
}

func (ctrl *SkillController) Get(c *gin.Context) {
	idSkill, err := strconv.ParseInt(c.Param("idSkill"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	skill, err := ctrl.skillService.LoadById(c.Request.Context(), idSkill)
	if err != nil {
		ctrl.respondSkillErr(c, err)
		return
	}
	c.JSON(http.StatusOK, skill)
}

func (ctrl *SkillController) Create(c *gin.Context) {
	var dto model.CreateSkillReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	skill, err := ctrl.skillService.Create(c.Request.Context(), dto)
	if err != nil {
		ctrl.respondSkillErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, skill)
}

func (ctrl *SkillController) Update(c *gin.Context) {
	idSkill, err := strconv.ParseInt(c.Param("idSkill"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var dto model.UpdateSkillReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := ctrl.skillService.Update(c.Request.Context(), idSkill, dto)
	if err != nil {
		ctrl.respondSkillErr(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (ctrl *SkillController) Delete(c *gin.Context) {
	idSkill, err := strconv.ParseInt(c.Param("idSkill"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := ctrl.skillService.Delete(c.Request.Context(), idSkill); err != nil {
		ctrl.respondSkillErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (ctrl *SkillController) Restore(c *gin.Context) {
	idSkill, err := strconv.ParseInt(c.Param("idSkill"), 10, 64)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	restored, err := ctrl.skillService.Restore(c.Request.Context(), idSkill)
	if err != nil {
		ctrl.respondSkillErr(c, err)
		return
	}
	c.JSON(http.StatusOK, restored)
}
