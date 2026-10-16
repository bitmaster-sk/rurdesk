package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingGateway struct {
	mu        sync.Mutex
	eventIds  []string
	failFirst int
}

func (g *recordingGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.eventIds = append(g.eventIds, r.Header.Get("X-Tracker-Event-Id"))
	if len(g.eventIds) <= g.failFirst {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (g *recordingGateway) receivedEventIds() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.eventIds...)
}

func startRecordingGateway(t *testing.T, failFirst int) (*recordingGateway, *model.AgentGateway) {
	recorder := &recordingGateway{failFirst: failFirst}
	server := httptest.NewServer(recorder)
	t.Cleanup(server.Close)
	return recorder, &model.AgentGateway{GatewayUrl: server.URL, WebhookSecret: []byte("secret")}
}

var testBackoffs = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}

func TestDeliverEventRetriesWithTheSameEventId(t *testing.T) {
	recorder, gateway := startRecordingGateway(t, 2)

	err := NewGatewayClient().DeliverEvent(context.Background(), gateway, WebhookEvent{IdRun: 1, Event: "cancelled"}, testBackoffs)

	require.NoError(t, err)
	ids := recorder.receivedEventIds()
	require.Len(t, ids, 3)
	assert.NotEmpty(t, ids[0])
	assert.Equal(t, ids[0], ids[1])
	assert.Equal(t, ids[0], ids[2])
}

func TestDeliverEventUsesANewEventIdPerDelivery(t *testing.T) {
	recorder, gateway := startRecordingGateway(t, 0)
	client := NewGatewayClient()
	event := WebhookEvent{IdRun: 1, Event: "cancelled"}

	require.NoError(t, client.DeliverEvent(context.Background(), gateway, event, testBackoffs))
	require.NoError(t, client.DeliverEvent(context.Background(), gateway, event, testBackoffs))

	ids := recorder.receivedEventIds()
	require.Len(t, ids, 2)
	assert.NotEqual(t, ids[0], ids[1])
}

func TestDeliverEventFailsAfterAllAttempts(t *testing.T) {
	recorder, gateway := startRecordingGateway(t, len(testBackoffs))

	err := NewGatewayClient().DeliverEvent(context.Background(), gateway, WebhookEvent{IdRun: 1, Event: "cancelled"}, testBackoffs)

	require.Error(t, err)
	assert.Len(t, recorder.receivedEventIds(), len(testBackoffs))
}
