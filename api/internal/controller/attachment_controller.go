package controller

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const attachmentMultipartOverheadBytes = 64 << 10

type AttachmentController struct {
	attachmentSvc *service.AttachmentService
}

func NewAttachmentController(attachmentSvc *service.AttachmentService) *AttachmentController {
	return &AttachmentController{attachmentSvc: attachmentSvc}
}

func (ac *AttachmentController) Upload(c *gin.Context) {
	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ac.attachmentSvc.MaxSizeBytes()+attachmentMultipartOverheadBytes)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			ResponseErr(c, errs.ErrAttachmentTooLarge)
			return
		}
		ResponseErr(c, errs.ErrBadRequest.WithMessage("multipart field file is required"))
		return
	}
	defer func() { _ = file.Close() }()

	scope, err := parseAttachmentScope(c)
	if err != nil {
		ResponseErr(c, err)
		return
	}

	attachment, err := ac.attachmentSvc.Upload(ctx, user, service.AttachmentUpload{
		Scope:    scope,
		FileName: header.Filename,
		Size:     header.Size,
		Content:  file,
	})
	if err != nil {
		ResponseErr(c, err)
		return
	}
	ResponseOk(c, model.AttachmentRes{
		IdAttachment: attachment.IdAttachment,
		FileName:     attachment.FileName,
		MimeType:     attachment.MimeType,
		Size:         attachment.Size,
	})
}

func (ac *AttachmentController) Download(c *gin.Context) {
	idAttachment, err := uuid.Parse(c.Param("id"))
	if err != nil {
		ResponseErr(c, errs.ErrBadRequest.WithMessage("invalid attachment id"))
		return
	}

	ctx := c.Request.Context()
	user, _ := extctx.GetUser(ctx)

	attachment, content, err := ac.attachmentSvc.Open(ctx, user, idAttachment)
	if err != nil {
		ResponseErr(c, err)
		return
	}
	defer func() { _ = content.Close() }()

	c.DataFromReader(http.StatusOK, attachment.Size, attachment.MimeType, content, map[string]string{
		"Content-Disposition":    attachmentDisposition(attachment),
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "private, max-age=31536000, immutable",
	})
}

func parseAttachmentScope(c *gin.Context) (model.AttachmentScope, error) {
	if rawIdSpace := c.Request.FormValue("idSpace"); rawIdSpace != "" {
		if c.Request.FormValue("idMessageRecipientType") != "" || c.Request.FormValue("idRecipient") != "" {
			return model.AttachmentScope{}, errs.ErrBadRequest.WithMessage("send either idSpace or idMessageRecipientType with idRecipient")
		}
		idSpace, err := strconv.ParseInt(rawIdSpace, 10, 64)
		if err != nil {
			return model.AttachmentScope{}, errs.ErrBadRequest.WithMessage("invalid idSpace")
		}
		return model.AttachmentScope{IdSpace: &idSpace}, nil
	}
	recipientType, err := strconv.ParseInt(c.Request.FormValue("idMessageRecipientType"), 10, 64)
	if err != nil {
		return model.AttachmentScope{}, errs.ErrBadRequest.WithMessage("idMessageRecipientType is required")
	}
	idRecipient, err := strconv.ParseInt(c.Request.FormValue("idRecipient"), 10, 64)
	if err != nil {
		return model.AttachmentScope{}, errs.ErrBadRequest.WithMessage("idRecipient is required")
	}
	scope, ok := model.AttachmentScopeOf(model.MessageRecipientType(recipientType), idRecipient)
	if !ok {
		return model.AttachmentScope{}, errs.ErrBadRequest.WithMessage("unknown idMessageRecipientType")
	}
	return scope, nil
}

func attachmentDisposition(attachment *model.Attachment) string {
	disposition := "attachment"
	if strings.HasPrefix(attachment.MimeType, "image/") || attachment.MimeType == "application/pdf" {
		disposition = "inline"
	}
	if formatted := mime.FormatMediaType(disposition, map[string]string{"filename": attachment.FileName}); formatted != "" {
		return formatted
	}
	return disposition
}
