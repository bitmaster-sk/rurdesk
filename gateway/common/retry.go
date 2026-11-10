package common

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

func RetryWithBackoff(ctx context.Context, name string, attempt func(context.Context) error) bool {
	const (
		maxAttempts = 12
		baseDelay   = 1 * time.Second
		maxDelay    = 30 * time.Second
	)
	delay := baseDelay
	for count := 1; ; count++ {
		err := attempt(ctx)
		if err == nil {
			return true
		}
		log.Warn().Err(err).Int("attempt", count).Msgf("%s report failed; will retry", name)
		if count >= maxAttempts {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(delay):
		}
		if delay < maxDelay {
			if delay *= 2; delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}
