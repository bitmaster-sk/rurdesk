package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

type IssueService struct {
	pool *pgxpool.Pool
}

func NewIssueService(pool *pgxpool.Pool) *IssueService {
	return &IssueService{pool: pool}
}

func (s *IssueService) StartIdempotencyCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.pool.Exec(ctx,
					`UPDATE issues.issue SET idempotency_key = NULL
					 WHERE idempotency_key IS NOT NULL
					   AND create_at < (now() AT TIME ZONE 'utc') - INTERVAL '24 hours'`,
				); err != nil {
					log.Warn().Err(err).Msg("idempotency cleanup failed")
				}
			}
		}
	}()
}
