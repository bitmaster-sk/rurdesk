package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/ai"
	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/viper"
)

// QualityService handles AI-driven issue quality checking.
type QualityService struct {
	aiClient    ai.Provider
	qualityRepo *repository.QualityRepository
	issueRepo   *repository.IssueRepository
}

func NewQualityService(
	aiClient ai.Provider,
	qualityRepo *repository.QualityRepository,
	issueRepo *repository.IssueRepository,
) *QualityService {
	return &QualityService{
		aiClient:    aiClient,
		qualityRepo: qualityRepo,
		issueRepo:   issueRepo,
	}
}

// contentHash hashes every input feeding the AI prompt — title, description, and the
// four metadata flags. Omitting the flags would let a metadata-only edit (e.g. setting
// severity) leave the hash unchanged, returning a stale cached report.
func contentHash(title, description string, hasAssignee, hasSeverity, hasState, hasEstimated bool) string {
	payload := fmt.Sprintf("%s\n%s\n%t|%t|%t|%t", title, description, hasAssignee, hasSeverity, hasState, hasEstimated)
	h := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("%x", h)
}

func (s *QualityService) callAI(ctx context.Context, title, description string, hasAssignee, hasSeverity, hasState, hasEstimated bool) (*model.QualityCheckRes, error) {
	messages := ai.BuildQualityPrompt(title, description, "", hasAssignee, hasSeverity, hasState, hasEstimated)

	qualityModel := viper.GetString("AI_QUALITY_MODEL")
	if qualityModel == "" {
		qualityModel = viper.GetString("AI_MODEL")
	}
	if qualityModel == "" {
		return nil, errs.ErrAiNotConfigured
	}

	res, err := s.aiClient.Complete(ctx, ai.CompletionReq{
		Model:     qualityModel,
		Messages:  messages,
		Tools:     ai.QualityTools(),
		MaxTokens: 32768,
	})
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).
			Str("model", qualityModel).
			Msg("AI quality: request failed")
		return nil, errs.ErrAiUnavailable
	}

	report, err := ai.ParseQualityResponse(res)
	if err != nil {
		extctx.GetLogger(ctx).Error().Err(err).
			Str("model", qualityModel).
			Str("stop_reason", res.StopReason).
			Str("raw_response", string(res.ToolUseInput)).
			Msg("AI quality: failed to parse response")
		return nil, errs.ErrAiInvalidResponse
	}

	return report, nil
}

// Preview runs an AI quality check without persisting — for pre-save use.
func (s *QualityService) Preview(ctx context.Context, title, description string) (*model.QualityCheckRes, error) {
	report, err := s.callAI(ctx, title, description, false, false, false, false)
	if err != nil {
		return nil, err
	}
	report.CheckedAt = time.Now().UTC()
	return report, nil
}

// Check runs an AI quality check for an existing issue and persists the result,
// skipping the AI call when the content hash is unchanged.
func (s *QualityService) Check(ctx context.Context, idIssue int64, title, description string, checkedBy int64) (*model.QualityCheckRes, error) {
	issue, err := s.issueRepo.LoadIssue(ctx, &repository.LoadIssueFilter{IdIssue: &idIssue})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}

	hasAssignee := issue.AssignedTo != nil
	hasSeverity := issue.IdSeverity != nil
	hasState := issue.IdState != nil
	hasEstimated := issue.Estimated > 0
	hash := contentHash(title, description, hasAssignee, hasSeverity, hasState, hasEstimated)

	existing, cachedResponse, err := s.qualityRepo.GetByIssue(ctx, idIssue)
	if err != nil {
		return nil, err
	}

	if existing != nil && existing.ContentHash == hash {
		return cachedResponse, nil
	}

	report, err := s.callAI(ctx, title, description, hasAssignee, hasSeverity, hasState, hasEstimated)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	report.CheckedAt = now

	quality := &model.IssueQuality{
		IdIssue:     idIssue,
		Score:       report.Score,
		ContentHash: hash,
		CheckedAt:   now,
		CheckedBy:   checkedBy,
	}

	if existing == nil {
		if insertErr := s.qualityRepo.Insert(ctx, quality, report); insertErr != nil {
			return nil, insertErr
		}
	} else {
		if updateErr := s.qualityRepo.Update(ctx, quality, report); updateErr != nil {
			return nil, updateErr
		}
	}

	return report, nil
}

// GetForIssue returns the persisted quality report for an issue, or errs.ErrNotFound if none exists.
func (s *QualityService) GetForIssue(ctx context.Context, idIssue int64) (*model.QualityCheckRes, error) {
	_, response, err := s.qualityRepo.GetByIssue(ctx, idIssue)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errs.ErrNotFound
	}
	return response, nil
}
