package controller

import (
	"errors"
	"net/http"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func ResponseOk(c *gin.Context, body any) {
	c.JSON(http.StatusOK, body)
}

func ResponseErr(c *gin.Context, err error) {
	var appErr *errs.Error
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
	case errs.As(err, &appErr):
		_ = c.Error(appErr)
		c.Status(appErr.HttpStatus())
	default:
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
	}
}
