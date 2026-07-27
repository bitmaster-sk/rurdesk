package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, target, nil)
	return c, w
}

func TestHealthLiveAlwaysReportsOk(t *testing.T) {
	ctrl := NewHealthController(nil)

	c, w := newTestContext(http.MethodGet, "/healthz")
	ctrl.Live(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestHealthReadyReturnsOkWhenAllDependenciesUp(t *testing.T) {
	ctrl := NewHealthController([]HealthCheck{
		{Name: "db", Ping: func(context.Context) error { return nil }},
		{Name: "cache", Ping: func(context.Context) error { return nil }},
	})

	c, w := newTestContext(http.MethodGet, "/healthz/ready")
	ctrl.Ready(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "ok", body.Status)
	assert.Equal(t, "up", body.Checks["db"])
	assert.Equal(t, "up", body.Checks["cache"])
}

func TestHealthReadyReturns503WhenADependencyIsDown(t *testing.T) {
	ctrl := NewHealthController([]HealthCheck{
		{Name: "db", Ping: func(context.Context) error { return nil }},
		{Name: "cache", Ping: func(context.Context) error { return errors.New("connection refused") }},
	})

	c, w := newTestContext(http.MethodGet, "/healthz/ready")
	ctrl.Ready(c)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "unavailable", body.Status)
	assert.Equal(t, "up", body.Checks["db"])
	assert.Equal(t, "down", body.Checks["cache"])
}
