package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/ai"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockAIProvider struct {
	response *ai.CompletionRes
	err      error
}

func (m *mockAIProvider) Complete(_ context.Context, _ ai.CompletionReq) (*ai.CompletionRes, error) {
	return m.response, m.err
}

func validSplitResponse() *ai.CompletionRes {
	payload := `{"children":[
		{"title":"Child task one","description":"First child task description."},
		{"title":"Child task two","description":"Second child task description."}
	]}`
	return &ai.CompletionRes{ToolUseInput: json.RawMessage(payload), StopReason: "tool_use"}
}

// With AI_MODEL unset, Preview must return errs.ErrAiNotConfigured up front —
// before any DB load or provider call (no hardcoded model default).
func TestPreview_NoModelConfigured_ReturnsErrAiNotConfigured(t *testing.T) {
	oldModel := viper.GetString("AI_MODEL")
	defer viper.Set("AI_MODEL", oldModel)
	viper.Set("AI_MODEL", "")

	// nil repos are safe: the guard returns before the issue is loaded.
	mock := &mockAIProvider{response: validSplitResponse()}
	svc := service.NewSplitService(nil, mock, nil, nil, nil)

	_, err := svc.Preview(context.Background(), 1, "")
	require.Error(t, err)
	assert.ErrorIs(t, err, errs.ErrAiNotConfigured)
}

// The empty-children guard must reject before touching the database.
func TestAccept_EmptyChildren_ReturnsBadRequest(t *testing.T) {
	mock := &mockAIProvider{response: validSplitResponse()}
	svc := service.NewSplitService(nil, mock, nil, nil, nil)

	_, err := svc.Accept(context.Background(), 1, 1, nil, 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, errs.ErrBadRequest)
}
