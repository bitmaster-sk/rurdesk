package handler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/flow"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/samlauth"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/gin-gonic/gin"
)

const metadataRetryDelay = 15 * time.Minute

var errIdpReturnedError = errors.New("identity provider returned an error")

type metadataRetry struct {
	mu        sync.Mutex
	notBefore time.Time
}

func (r *metadataRetry) isDue(now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !now.Before(r.notBefore)
}

func (r *metadataRetry) postpone(now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notBefore = now.Add(metadataRetryDelay)
}

func (h *Handler) authUrl(ctx context.Context, connection config.Connection, baseUrl string, state flow.State) (string, error) {
	if connection.Protocol == config.ProtocolSaml {
		client, err := h.samlClient(ctx, connection, baseUrl)
		if err != nil {
			return "", err
		}
		request, err := client.NewRequest()
		if err != nil {
			return "", err
		}
		state.RequestId = request.Id
		key, err := h.deps.Flows.SaveState(ctx, state)
		if err != nil {
			return "", err
		}
		return client.RedirectUrl(request, key)
	}
	client, err := h.deps.Clients.Client(ctx, connection, baseUrl+config.CallbackPath)
	if err != nil {
		return "", err
	}
	key, err := h.deps.Flows.SaveState(ctx, state)
	if err != nil {
		return "", err
	}
	return client.AuthUrl(key, state.Nonce, state.Verifier), nil
}

func (h *Handler) identity(c *gin.Context, state flow.State, connection config.Connection, baseUrl string) (identity.Identity, error) {
	ctx := c.Request.Context()
	if connection.Protocol == config.ProtocolSaml {
		client, err := h.samlClient(ctx, connection, baseUrl)
		if err != nil {
			return identity.Identity{}, err
		}
		return client.Identity(c.Request, state.RequestId)
	}
	if idpError := c.Query("error"); idpError != "" {
		return identity.Identity{}, fmt.Errorf("%w: %s %s", errIdpReturnedError, idpError, c.Query("error_description"))
	}
	client, err := h.deps.Clients.Client(ctx, connection, baseUrl+config.CallbackPath)
	if err != nil {
		return identity.Identity{}, err
	}
	return client.Identity(ctx, c.Query("code"), state.Verifier, state.Nonce)
}

func (h *Handler) samlClient(ctx context.Context, connection config.Connection, baseUrl string) (*samlauth.Client, error) {
	pair, err := h.deps.Config.KeyPair(ctx)
	if err != nil {
		return nil, err
	}
	return samlauth.NewClient(connection, pair, baseUrl, h.deps.Replay)
}

func (h *Handler) activeConnection(ctx context.Context) (config.Connection, error) {
	connection, err := h.deps.Config.Active(ctx)
	now := time.Now()
	if err != nil || !connection.NeedsMetadataRefresh(now) || !h.metadataRetry.isDue(now) {
		return connection, err
	}
	metadataXml, err := samlauth.FetchMetadata(ctx, h.deps.HttpClient, connection.MetadataUrl)
	if err == nil {
		err = h.deps.Config.SaveMetadata(ctx, true, metadataXml)
	}
	if err != nil {
		h.metadataRetry.postpone(now)
		extctx.GetLogger(ctx).Warn().Err(err).Msg("sso: refreshing the identity provider metadata failed, the last copy stays in use")
		return connection, nil
	}
	connection.MetadataXml = metadataXml
	return connection, nil
}
