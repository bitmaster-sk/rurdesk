package controller

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/gin-gonic/gin"
)

// BotGatewayController manages the 1:1 bot→gateway record. All endpoints sit
// behind the AdminOnly middleware; handlers only validate the target user.
type BotGatewayController struct {
	botGwRepo *repository.BotGatewayRepository
	userRepo  *repository.UserRepository
}

func NewBotGatewayController(
	botGwRepo *repository.BotGatewayRepository,
	userRepo *repository.UserRepository,
) *BotGatewayController {
	return &BotGatewayController{
		botGwRepo: botGwRepo,
		userRepo:  userRepo,
	}
}

// requireBot aborts with 422 unless the target user is a bot.
func (ctrl *BotGatewayController) requireBot(c *gin.Context) (int64, bool) {
	idUser, err := strconv.ParseInt(c.Param("idUser"), 10, 64)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return 0, false
	}
	isBot, err := ctrl.userRepo.IsBotUser(c.Request.Context(), idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return 0, false
	}
	if !isBot {
		_ = c.Error(errs.ErrNotABot)
		c.Status(http.StatusUnprocessableEntity)
		return 0, false
	}
	return idUser, true
}

func generateWebhookSecret() ([]byte, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func (ctrl *BotGatewayController) GetBotGateway(c *gin.Context) {
	idUser, ok := ctrl.requireBot(c)
	if !ok {
		return
	}
	gw, err := ctrl.botGwRepo.LoadByBotUser(c.Request.Context(), idUser)
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, gw)
}

func (ctrl *BotGatewayController) CreateBotGateway(c *gin.Context) {
	idUser, ok := ctrl.requireBot(c)
	if !ok {
		return
	}

	var req model.CreateBotGatewayReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	gw, err := ctrl.botGwRepo.Insert(c.Request.Context(), idUser, req, secret)
	if isConflict(err) {
		_ = c.Error(errs.ErrGatewayExists)
		c.Status(http.StatusConflict)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, &model.CreateBotGatewayRes{
		BotGateway:            *gw,
		TrackerToGatewayToken: hex.EncodeToString(secret),
	})
}

// UpdateBotGateway changes the gateway URL of an existing record (admin edit).
// The webhook secret is untouched — no new token is issued.
func (ctrl *BotGatewayController) UpdateBotGateway(c *gin.Context) {
	idUser, ok := ctrl.requireBot(c)
	if !ok {
		return
	}

	var req model.CreateBotGatewayReq
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(err)
		c.Status(http.StatusBadRequest)
		return
	}

	gw, err := ctrl.botGwRepo.UpdateUrl(c.Request.Context(), idUser, req.GatewayUrl)
	if errors.Is(err, repository.ErrBotGatewayNotFound) {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gw)
}

// RegenerateGatewayToken replaces the gateway's webhook signing secret and
// returns the new one-time token. The old secret stops verifying immediately.
func (ctrl *BotGatewayController) RegenerateGatewayToken(c *gin.Context) {
	idUser, ok := ctrl.requireBot(c)
	if !ok {
		return
	}

	secret, err := generateWebhookSecret()
	if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	gw, err := ctrl.botGwRepo.UpdateSecret(c.Request.Context(), idUser, secret)
	if errors.Is(err, repository.ErrBotGatewayNotFound) {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, &model.CreateBotGatewayRes{
		BotGateway:            *gw,
		TrackerToGatewayToken: hex.EncodeToString(secret),
	})
}

func (ctrl *BotGatewayController) DeleteBotGateway(c *gin.Context) {
	idUser, ok := ctrl.requireBot(c)
	if !ok {
		return
	}
	err := ctrl.botGwRepo.DeleteByBotUser(c.Request.Context(), idUser)
	if errors.Is(err, repository.ErrBotGatewayNotFound) {
		_ = c.Error(errs.ErrNotFound)
		c.Status(http.StatusNotFound)
		return
	} else if err != nil {
		_ = c.Error(err)
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}
