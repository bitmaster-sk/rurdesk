package middleware

import (
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/gin-gonic/gin"
)

// ErrorRenderer gives every failed request a uniform, translatable JSON body.
//
// Handlers signal failure by attaching the error to gin's chain (`c.Error(err)`)
// and setting a status (`c.Status(code)`), then returning — they do not write a
// body. After the handler runs, this middleware renders that body once:
//   - if a `*errs.Error` was attached, it is used verbatim ({code, message, translateKey});
//   - otherwise the status is mapped to the matching sentinel via errs.FromStatus,
//     so even a bare status yields a code the i18n frontend can localize.
//
// It never touches success responses (no errors) or handlers that already wrote a
// body themselves (c.Writer.Written()), so endpoints rendering their own JSON are
// left alone.
func ErrorRenderer() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}

		status := c.Writer.Status()
		if status < http.StatusBadRequest {
			status = http.StatusInternalServerError
		}

		var appErr *errs.Error
		for _, e := range c.Errors {
			if errs.As(e.Err, &appErr) {
				break
			}
		}
		if appErr == nil {
			appErr = errs.FromStatus(status)
		}

		c.JSON(status, appErr)
	}
}
