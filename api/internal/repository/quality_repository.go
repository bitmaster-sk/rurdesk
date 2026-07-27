package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/extctx"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QualityRepository manages the issues.issue_quality table.
type QualityRepository struct {
	pool *pgxpool.Pool
}

func NewQualityRepository(pool *pgxpool.Pool) *QualityRepository {
	return &QualityRepository{pool: pool}
}

// storedReport mirrors the JSON structure persisted in the report column.
type storedReport struct {
	Score       int                       `json:"score"`
	Dimensions  model.QualityDimensions   `json:"dimensions"`
	Problems    []string                  `json:"problems"`
	Suggestions []model.QualitySuggestion `json:"suggestions"`
}

// GetByIssue returns (nil, nil, nil) if no record exists for the issue.
func (r *QualityRepository) GetByIssue(ctx context.Context, idIssue int64) (*model.IssueQuality, *model.QualityCheckRes, error) {
	db := extctx.GetDb(ctx, r.pool)

	var (
		score       int
		reportJSON  []byte
		contentHash string
		checkedAt   time.Time
		checkedBy   int64
	)

	err := db.QueryRow(ctx, `
		SELECT score, report, content_hash, checked_at, checked_by
		FROM issues.issue_quality
		WHERE id_issue = $1
	`, idIssue).Scan(&score, &reportJSON, &contentHash, &checkedAt, &checkedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("querying issue quality: %w", err)
	}

	var report storedReport
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		return nil, nil, fmt.Errorf("quality repository: unmarshal report: %w", err)
	}

	quality := &model.IssueQuality{
		IdIssue:     idIssue,
		Score:       score,
		ContentHash: contentHash,
		CheckedAt:   checkedAt,
		CheckedBy:   checkedBy,
	}

	response := &model.QualityCheckRes{
		Score:       report.Score,
		Dimensions:  report.Dimensions,
		Problems:    report.Problems,
		Suggestions: report.Suggestions,
		CheckedAt:   checkedAt,
		FromCache:   true,
	}

	return quality, response, nil
}

func (r *QualityRepository) Insert(ctx context.Context, quality *model.IssueQuality, response *model.QualityCheckRes) error {
	db := extctx.GetDb(ctx, r.pool)

	reportJSON, err := json.Marshal(storedReport{
		Score:       response.Score,
		Dimensions:  response.Dimensions,
		Problems:    response.Problems,
		Suggestions: response.Suggestions,
	})
	if err != nil {
		return fmt.Errorf("quality repository: marshal report: %w", err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO issues.issue_quality (id_issue, score, report, content_hash, checked_by)
		VALUES ($1, $2, $3, $4, $5)
	`, quality.IdIssue, quality.Score, reportJSON, quality.ContentHash, quality.CheckedBy)
	if err != nil {
		return fmt.Errorf("inserting issue quality: %w", err)
	}
	return nil
}

func (r *QualityRepository) Update(ctx context.Context, quality *model.IssueQuality, response *model.QualityCheckRes) error {
	db := extctx.GetDb(ctx, r.pool)

	reportJSON, err := json.Marshal(storedReport{
		Score:       response.Score,
		Dimensions:  response.Dimensions,
		Problems:    response.Problems,
		Suggestions: response.Suggestions,
	})
	if err != nil {
		return fmt.Errorf("quality repository: marshal report: %w", err)
	}

	_, err = db.Exec(ctx, `
		UPDATE issues.issue_quality
		SET score = $2, report = $3, content_hash = $4,
		    checked_at = (now() AT TIME ZONE 'utc'), checked_by = $5
		WHERE id_issue = $1
	`, quality.IdIssue, quality.Score, reportJSON, quality.ContentHash, quality.CheckedBy)
	if err != nil {
		return fmt.Errorf("updating issue quality: %w", err)
	}
	return nil
}

func (r *QualityRepository) Delete(ctx context.Context, idIssue int64) error {
	db := extctx.GetDb(ctx, r.pool)
	_, err := db.Exec(ctx, `DELETE FROM issues.issue_quality WHERE id_issue = $1`, idIssue)
	if err != nil {
		return fmt.Errorf("deleting issue quality: %w", err)
	}
	return nil
}
