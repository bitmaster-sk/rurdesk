package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/go-redis/redis/v8"
)

const (
	plantUmlCachePrefix = "plantuml:svg:"
	plantUmlCacheTTL    = 7 * 24 * time.Hour
	plantUmlMaxSvgBytes = 2 << 20
)

type PlantUmlService struct {
	settings *AppSettingsService
	cache    *redis.Client
	client   *http.Client
}

func NewPlantUmlService(settings *AppSettingsService, cache *redis.Client) *PlantUmlService {
	return &PlantUmlService{
		settings: settings,
		cache:    cache,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *PlantUmlService) Render(ctx context.Context, source string) (string, error) {
	serverUrl := s.settings.PlantUmlServerUrl()
	if serverUrl == "" {
		return "", errs.ErrPlantUmlDisabled
	}
	if !strings.Contains(strings.TrimSpace(source), "@start") {
		source = "@startuml\n" + source + "\n@enduml"
	}
	key := plantUmlCachePrefix + hashPlantUmlCacheKey(serverUrl, source)
	if svg, err := s.cache.Get(ctx, key).Result(); err == nil {
		return svg, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serverUrl+"/svg", strings.NewReader(source))
	if err != nil {
		return "", errs.ErrPlantUmlUnavailable
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", errs.ErrPlantUmlUnavailable
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, plantUmlMaxSvgBytes+1))
	if err != nil {
		return "", errs.ErrPlantUmlUnavailable
	}
	if len(body) > plantUmlMaxSvgBytes {
		return "", errs.ErrPlantUmlUnavailable
	}
	switch resp.StatusCode {
	case http.StatusOK:
		svg := string(body)
		_ = s.cache.Set(ctx, key, svg, plantUmlCacheTTL).Err()
		return svg, nil
	case http.StatusBadRequest:
		message := resp.Header.Get("X-PlantUML-Diagram-Error")
		if message == "" {
			message = errs.ErrPlantUmlSyntax.Message
		}
		return "", errs.ErrPlantUmlSyntax.WithMessage(message)
	default:
		return "", errs.ErrPlantUmlUnavailable
	}
}

func hashPlantUmlCacheKey(serverUrl, source string) string {
	sum := sha256.Sum256([]byte(serverUrl + "\n" + source))
	return hex.EncodeToString(sum[:])
}
