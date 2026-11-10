package goose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/rs/zerolog/log"
)

const (
	ollamaProvider    = "ollama"
	ollamaDefaultHost = "http://localhost:11434"
	ollamaShowTimeout = 5 * time.Second
)

func ollamaThinkingByModel(ctx context.Context, host string, models []common.Model) (map[string]bool, []common.Model) {
	client := &http.Client{Timeout: ollamaShowTimeout}
	isThinkingByModel := map[string]bool{}
	unanswered := []common.Model{}
	for _, model := range models {
		if model.Provider != ollamaProvider {
			continue
		}
		capabilities, err := ollamaCapabilities(ctx, client, host, model.Name)
		if err != nil {
			log.Warn().Err(err).Str("model", model.Id()).Msg("reading the ollama model capabilities")
			unanswered = append(unanswered, model)
			continue
		}
		if capabilities != nil {
			isThinkingByModel[model.Id()] = slices.Contains(capabilities, "thinking")
		}
	}
	return isThinkingByModel, unanswered
}

func ollamaCapabilities(ctx context.Context, client *http.Client, host, name string) ([]string, error) {
	body, err := json.Marshal(map[string]string{"model": name})
	if err != nil {
		return nil, fmt.Errorf("encoding the show request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, ollamaBaseUrl(host)+"/api/show", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building the show request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("calling ollama: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama answered HTTP %d", response.StatusCode)
	}
	var show struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(response.Body).Decode(&show); err != nil {
		return nil, fmt.Errorf("decoding the show response: %w", err)
	}
	return show.Capabilities, nil
}

func ollamaBaseUrl(host string) string {
	if host == "" {
		return ollamaDefaultHost
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return strings.TrimRight(host, "/")
}
