package service_test

import (
	"context"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// With neither AI_QUALITY_MODEL nor AI_MODEL set, Preview must return
// errs.ErrAiNotConfigured up front — no hardcoded model default, no provider call.
func TestQualityPreview_NoModelConfigured_ReturnsErrAiNotConfigured(t *testing.T) {
	oldModel, oldQuality := viper.GetString("AI_MODEL"), viper.GetString("AI_QUALITY_MODEL")
	defer func() { viper.Set("AI_MODEL", oldModel); viper.Set("AI_QUALITY_MODEL", oldQuality) }()
	viper.Set("AI_MODEL", "")
	viper.Set("AI_QUALITY_MODEL", "")

	// provider would return a valid response if (wrongly) called — the guard must
	// short-circuit before that, so the error, not the response, proves the guard.
	mock := &mockAIProvider{response: validSplitResponse()}
	svc := service.NewQualityService(mock, nil, nil)

	_, err := svc.Preview(context.Background(), "Some title", "Some description")
	require.Error(t, err)
	assert.ErrorIs(t, err, errs.ErrAiNotConfigured)
}

// AI_QUALITY_MODEL alone (AI_MODEL empty) is enough for Preview to call the provider.
func TestQualityPreview_QualityModelSet_DoesNotShortCircuit(t *testing.T) {
	oldModel, oldQuality := viper.GetString("AI_MODEL"), viper.GetString("AI_QUALITY_MODEL")
	defer func() { viper.Set("AI_MODEL", oldModel); viper.Set("AI_QUALITY_MODEL", oldQuality) }()
	viper.Set("AI_MODEL", "")
	viper.Set("AI_QUALITY_MODEL", "some-quality-model")

	mock := &mockAIProvider{err: errs.ErrAiUnavailable} // proves the provider WAS called
	svc := service.NewQualityService(mock, nil, nil)

	_, err := svc.Preview(context.Background(), "Some title", "Some description")
	require.Error(t, err)
	assert.NotErrorIs(t, err, errs.ErrAiNotConfigured)
}
