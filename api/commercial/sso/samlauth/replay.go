package samlauth

import (
	"context"
	"fmt"
	"time"

	"github.com/crewjam/saml"
	"github.com/go-redis/redis/v8"
)

const (
	replayPrefix = "sso:saml:assertion:"
	minReplayTtl = time.Minute
)

type ReplayCache struct {
	cache *redis.Client
}

var _ saml.AssertionReplayCache = (*ReplayCache)(nil)

func NewReplayCache(cache *redis.Client) *ReplayCache {
	return &ReplayCache{cache: cache}
}

func (r *ReplayCache) Consume(assertionId string, expiresAt time.Time) error {
	ttl := max(time.Until(expiresAt), minReplayTtl)
	isNew, err := r.cache.SetNX(context.Background(), replayPrefix+assertionId, 1, ttl).Result()
	if err != nil {
		return fmt.Errorf("recording saml assertion: %w", err)
	}
	if !isNew {
		return saml.ErrAssertionReplayed
	}
	return nil
}
