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

func TestFetchFileContent(t *testing.T) {
	testCases := []struct {
		name        string
		status      int
		body        []byte
		wantContent string
		wantErr     error
	}{
		{name: "returns text content", status: http.StatusOK, body: []byte("hello\n"), wantContent: "hello\n"},
		{name: "rejects content over the size limit", status: http.StatusOK, body: make([]byte, maxFileContentBytes+1), wantErr: ErrFileTooLarge},
		{name: "rejects binary content", status: http.StatusOK, body: []byte{0xff, 0xfe, 0x00}, wantErr: ErrBinaryFile},
		{name: "fails on a non-200 response", status: http.StatusNotFound, body: []byte("not found")},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(testCase.status)
				_, _ = w.Write(testCase.body)
			}))
			defer srv.Close()

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
			require.NoError(t, err)
			content, err := fetchFileContent(t.Context(), srv.Client(), req, "f.txt", "ref")

			switch {
			case testCase.wantErr != nil:
				require.ErrorIs(t, err, testCase.wantErr)
			case testCase.status != http.StatusOK:
				require.ErrorContains(t, err, "404")
			default:
				require.NoError(t, err)
				assert.Equal(t, testCase.wantContent, string(content))
			}
		})
	}
}
