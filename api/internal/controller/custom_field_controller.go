package controller

import (
	"context"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type CustomFieldController struct {
	customFieldRepo *repository.CustomFieldRepository
	customFieldSvc  *service.CustomFieldService
	acl             *service.AclService
	notifier        *notify.Notifier
	projectRepo     *repository.ProjectRepository
}

func NewCustomFieldController(
	customFieldRepo *repository.CustomFieldRepository,
	customFieldSvc *service.CustomFieldService,
	acl *service.AclService,
	notifier *notify.Notifier,
	projectRepo *repository.ProjectRepository,
) *CustomFieldController {
	return &CustomFieldController{
		customFieldRepo: customFieldRepo,
		customFieldSvc:  customFieldSvc,
		acl:             acl,
		notifier:        notifier,
		projectRepo:     projectRepo,
	}
}

// On a delete the field must be the definition loaded before the row went away:
// the client drops the values by its key.
func (cfc *CustomFieldController) BroadcastCustomField(
	ctx context.Context,
	field *model.CustomField,
	action notify.NoticeAction,
) {
	if cfc.notifier == nil || field == nil {
		return
	}
	members, err := cfc.projectRepo.LoadProjectsMembers(ctx, []int64{field.IdProject})
	if err != nil || len(members) == 0 {
		return
	}
	idsUser := make([]int64, len(members))
	for i, member := range members {
		idsUser[i] = member.IdUser
	}
	cfc.notifier.Send <- &notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectCustomField,
		Action:  action,
		Payload: field,
	}
}

func (cfc *CustomFieldController) GetCustomFields(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	idsProject, err := cfc.acl.LoadVisibleProjectIds(ctx, user.IdUser)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if len(idsProject) == 0 {
		ResponseOk(c, []*model.CustomField{})
		return
	}
	fields, err := cfc.customFieldRepo.LoadCustomFields(ctx, idsProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, fields)
}

func (cfc *CustomFieldController) CreateCustomField(c *gin.Context) {
	var dto model.CreateCustomFieldReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !cfc.acl.CanManageCustomField(ctx, user.IdUser, dto.IdProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	field, err := cfc.customFieldSvc.Create(ctx, dto)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	cfc.BroadcastCustomField(ctx, field, notify.ActionCreate)
	ResponseOk(c, field)
}

func (cfc *CustomFieldController) EditCustomField(c *gin.Context) {
	idCustomField, err := strconv.ParseInt(c.Param("idCustomField"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}
	var dto model.EditCustomFieldReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	dto.IdCustomField = idCustomField

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !cfc.acl.CanManageCustomField(ctx, user.IdUser, dto.IdProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	intent := service.OptionIntent{DeleteValues: c.Query("deleteOptionValues") == "true"}
	if raw, has := c.GetQuery("migrateOptionTo"); has {
		target, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil {
			ResponseErr(c, errs.ErrBadRequest)
			return
		}
		intent.MigrateTo = &target
	}

	field, err := cfc.customFieldSvc.Update(ctx, dto, intent)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	cfc.BroadcastCustomField(ctx, field, notify.ActionUpdate)
	ResponseOk(c, field)
}

func (cfc *CustomFieldController) GetCustomFieldUsage(c *gin.Context) {
	idProject, idCustomField, ok := cfc.parseParams(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !cfc.acl.CanManageCustomField(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}
	usage, err := cfc.customFieldRepo.LoadCustomFieldUsage(ctx, idProject, idCustomField)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, usage)
}

func (cfc *CustomFieldController) DeleteCustomField(c *gin.Context) {
	idProject, idCustomField, ok := cfc.parseParams(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !cfc.acl.CanManageCustomField(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	deleted, loadErr := cfc.customFieldRepo.LoadCustomField(ctx, idProject, idCustomField)
	if loadErr != nil {
		ResponseErr(c, loadErr)
		return
	}

	err := cfc.customFieldSvc.DeleteWithConfirm(ctx, idProject, idCustomField, c.Query("deleteValues") == "true")
	if err != nil {
		ResponseErr(c, err)
		return
	}
	cfc.BroadcastCustomField(ctx, deleted, notify.ActionDelete)
	c.Status(http.StatusOK)
}

func (cfc *CustomFieldController) parseParams(c *gin.Context) (int64, int64, bool) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return 0, 0, false
	}
	idCustomField, err := strconv.ParseInt(c.Param("idCustomField"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return 0, 0, false
	}
	return idProject, idCustomField, true
}
