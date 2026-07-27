package common

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// HeartbeatLoop sends a heartbeat to the tracker every 30s for the active
// agent_task, stopping when the context is cancelled.
type HeartbeatLoop struct {
	trackerClient *TrackerClient
}

func NewHeartbeatLoop(trackerClient *TrackerClient) *HeartbeatLoop {
	return &HeartbeatLoop{trackerClient: trackerClient}
}

func (h *HeartbeatLoop) Start(ctx context.Context, idTask int64) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.trackerClient.SendTaskHeartbeat(ctx, idTask); err != nil {
				log.Warn().Int64("idTask", idTask).Err(err).Msg("heartbeat failed")
			}
		}
	}
}
