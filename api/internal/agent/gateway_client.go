package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type WebhookEvent struct {
	IdRun     int64          `json:"idRun"`
	IdProject int64          `json:"idProject"`
	IdIssue   int64          `json:"idIssue"`
	IdUserBot int64          `json:"idUserBot"`
	Event     string         `json:"event"`
	Payload   map[string]any `json:"payload"`
	Sequence  int64          `json:"-"`
}

type GatewayClient struct {
	httpClient *http.Client
}

func NewGatewayClient() *GatewayClient {
	return &GatewayClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GatewayClient) SendEvent(ctx context.Context, gateway *model.BotGateway, event WebhookEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshalling event: %w", err)
	}

	ts := time.Now().Unix()
	signature := SignPayload(gateway.WebhookSecret, ts, body)
	eventId := uuid.New().String()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gateway.GatewayUrl+"/event", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tracker-Signature", signature)
	req.Header.Set("X-Tracker-Event-Id", eventId)
	req.Header.Set("X-Tracker-Sequence", fmt.Sprintf("%d", event.Sequence))
	req.Header.Set("X-Tracker-Event-Type", event.Event)

	log.Info().Msgf("Sending event to gateway: method=%s url=%s event=%s idRun=%d", req.Method, req.URL, event.Event, event.IdRun)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gateway returned status %d", resp.StatusCode)
	}
	return nil
}
