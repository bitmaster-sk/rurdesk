package githost

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostJSON(t *testing.T) {
	var gotMethod, gotAuth, gotContentType string
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("X-Auth")
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := postJSON(t.Context(), client, srv.URL, map[string]string{"k": "v"}, func(req *http.Request) {
		req.Header.Set("X-Auth", "secret")
	})
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, "secret", gotAuth)
	assert.Equal(t, "v", gotBody["k"])
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
