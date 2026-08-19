package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// Dispatcher dispatches MCP tool calls through the Gin engine in-process.
// No direct DB access — every call goes through the full middleware chain.
type Dispatcher struct {
	engine *gin.Engine
}

func NewDispatcher(engine *gin.Engine) *Dispatcher {
	return &Dispatcher{engine: engine}
}

// RequestOpts configures a single dispatcher call.
type RequestOpts struct {
	Method         string
	Path           string
	Body           any               // JSON-serializable; nil for GET/DELETE
	Query          map[string]string // query parameters
	Bearer         string            // "Bearer <token>" copied from MCP SSE connection
	ToolName       string            // for X-MCP-Tool header
	IdempotencyKey string            // optional
}

// Response is the result of a dispatcher call.
type Response struct {
	StatusCode   int
	Body         json.RawMessage
	Headers      http.Header
	IsError      bool
	IsAuthError  bool   // true on HTTP 401 — caller should close SSE session
	ErrorMessage string // populated when IsError is true
}

// Request builds an HTTP request, dispatches it through the Gin engine, and returns
// the parsed response. bearer is copied from the MCP client's SSE connection.
func (d *Dispatcher) Request(ctx context.Context, opts RequestOpts) (*Response, error) {
	var bodyReader io.Reader
	if opts.Body != nil {
		bodyBytes, err := json.Marshal(opts.Body)
		if err != nil {
			return nil, fmt.Errorf("marshalling request body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.Path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	// MCP clients send "Authorization: Bearer <key>", but auth middleware reads
	// the header raw and matches keys by shape (64 hex chars). Strip the prefix.
	authValue := opts.Bearer
	if len(authValue) > 7 && strings.EqualFold(authValue[:7], "Bearer ") {
		authValue = authValue[7:]
	}

	req.Header.Set("Authorization", authValue)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-MCP-Tool", opts.ToolName)
	req.Header.Set("X-MCP-Origin", "mcp")

	if opts.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", opts.IdempotencyKey)
	}

	if len(opts.Query) > 0 {
		query := req.URL.Query()
		for key, value := range opts.Query {
			query.Set(key, value)
		}
		req.URL.RawQuery = query.Encode()
	}

	recorder := httptest.NewRecorder()
	d.engine.ServeHTTP(recorder, req)

	return mapResponse(recorder, opts.ToolName)
}

// The caller should close the SSE session in this case.
func mapResponse(recorder *httptest.ResponseRecorder, toolName string) (*Response, error) {
	result := recorder.Result()
	defer result.Body.Close()

	rawBody, _ := io.ReadAll(result.Body)
	status := result.StatusCode

	body := json.RawMessage("null")
	if len(rawBody) > 0 {
		body = json.RawMessage(rawBody)
	}

	switch {
	case status == http.StatusOK || status == http.StatusCreated || status == http.StatusNoContent:
		return &Response{
			StatusCode: status,
			Body:       body,
			Headers:    result.Header,
		}, nil

	case status == http.StatusBadRequest:
		return &Response{
			StatusCode:   status,
			Body:         body,
			Headers:      result.Header,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("invalid request: %s", extractMessage(rawBody)),
		}, nil

	case status == http.StatusUnauthorized:
		return &Response{
			StatusCode:   status,
			Body:         body,
			Headers:      result.Header,
			IsError:      true,
			IsAuthError:  true,
			ErrorMessage: "authentication failed",
		}, nil

	case status == http.StatusForbidden:
		return &Response{
			StatusCode:   status,
			Body:         body,
			Headers:      result.Header,
			IsError:      true,
			ErrorMessage: "forbidden",
		}, nil

	case status == http.StatusNotFound:
		return &Response{
			StatusCode:   status,
			Body:         body,
			Headers:      result.Header,
			IsError:      true,
			ErrorMessage: "not found",
		}, nil

	case status == http.StatusConflict:
		return &Response{
			StatusCode:   status,
			Body:         body,
			Headers:      result.Header,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("conflict: %s", extractMessage(rawBody)),
		}, nil

	case status == http.StatusUnprocessableEntity:
		return &Response{
			StatusCode:   status,
			Body:         body,
			Headers:      result.Header,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("unprocessable: %s", extractMessage(rawBody)),
		}, nil

	case status == http.StatusTooManyRequests:
		retryAfter := result.Header.Get("Retry-After")
		msg := "rate limited"
		if retryAfter != "" {
			msg = fmt.Sprintf("rate limited, retry in %ss", retryAfter)
		}
		return &Response{
			StatusCode:   status,
			Body:         body,
			Headers:      result.Header,
			IsError:      true,
			ErrorMessage: msg,
		}, nil

	default:
		return &Response{
			StatusCode:   status,
			Body:         body,
			Headers:      result.Header,
			IsError:      true,
			ErrorMessage: fmt.Sprintf("server error: %d", status),
		}, nil
	}
}

// extractMessage pulls a human-readable message from an error response body.
func extractMessage(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var obj struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if obj.Error != "" {
			return obj.Error
		}
		if obj.Message != "" {
			return obj.Message
		}
	}
	s := strings.TrimSpace(string(raw))
	if utf8.RuneCountInString(s) > 200 {
		return string([]rune(s)[:200])
	}
	return s
}
