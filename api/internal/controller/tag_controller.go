package controller

import (
	"context"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
)

type TagController struct {
	tagRepo          *repository.TagRepository
	tagSvc           *service.TagService
	acl              *service.AclService
	notifier         *notify.Notifier
	projectMemberSvc *service.ProjectMemberService
}

func NewTagController(
	tagRepo *repository.TagRepository,
	tagSvc *service.TagService,
	acl *service.AclService,
	notifier *notify.Notifier,
	projectMemberSvc *service.ProjectMemberService,
) *TagController {
	return &TagController{
		tagRepo:          tagRepo,
		tagSvc:           tagSvc,
		acl:              acl,
		notifier:         notifier,
		projectMemberSvc: projectMemberSvc,
	}
}

func (tc *TagController) BroadcastTag(ctx context.Context, tag *model.Tag, action notify.NoticeAction) {
	if tc.notifier == nil || tag == nil {
		return
	}
	idsUser, err := tc.projectMemberSvc.LoadMemberIds(ctx, tag.IdProject)
	if err != nil || len(idsUser) == 0 {
		return
	}
	tc.notifier.Send <- &notify.Notice{
		IdsUser: idsUser,
		Subject: notify.SubjectTag,
		Action:  action,
		Payload: tag,
	}
}

func (tc *TagController) GetTags(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	idsProject, err := tc.acl.LoadVisibleProjectIds(ctx, user.IdUser)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	if len(idsProject) == 0 {
		ResponseOk(c, []*model.Tag{})
		return
	}
	tags, err := tc.tagRepo.LoadTags(ctx, idsProject)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, tags)
}

func (tc *TagController) CreateTag(c *gin.Context) {
	var dto model.CreateTagReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !tc.acl.CanManageTag(ctx, user.IdUser, dto.IdProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	tag, err := tc.tagSvc.Create(ctx, dto)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	tc.BroadcastTag(ctx, tag, notify.ActionCreate)
	ResponseOk(c, tag)
}

func (tc *TagController) EditTag(c *gin.Context) {
	idTag, err := strconv.ParseInt(c.Param("idTag"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}
	var dto model.EditTagReq
	if err := c.ShouldBindJSON(&dto); err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage(err.Error()))
		return
	}
	dto.IdTag = idTag

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !tc.acl.CanManageTag(ctx, user.IdUser, dto.IdProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	tag, err := tc.tagSvc.Update(ctx, dto)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	tc.BroadcastTag(ctx, tag, notify.ActionUpdate)
	ResponseOk(c, tag)
}

func (tc *TagController) DeleteTag(c *gin.Context) {
	idProject, err := strconv.ParseInt(c.Param("idProject"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}
	idTag, err := strconv.ParseInt(c.Param("idTag"), 10, 64)
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest)
		return
	}
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)
	if !tc.acl.CanManageTag(ctx, user.IdUser, idProject) {
		ResponseErr(c, errs.ErrForbidden)
		return
	}

	tag, err := tc.tagSvc.Delete(ctx, idProject, idTag)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	tc.BroadcastTag(ctx, tag, notify.ActionDelete)
	ResponseOk(c, tag)
}
