package controller

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionGetReturnsBuildIdentity(t *testing.T) {
	ctrl := NewVersionController()

	c, w := newTestContext(http.MethodGet, "/api/private/admin/version")
	ctrl.Get(c)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	// An unstamped test binary reports the dev defaults.
	assert.Equal(t, "dev", body.Version)
	assert.Equal(t, "unknown", body.Commit)
}
