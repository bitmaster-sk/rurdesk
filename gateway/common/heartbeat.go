package common

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// HeartbeatLoop sends a heartbeat to the tracker every interval for the active
// agent_task, stopping when the context is cancelled.
type HeartbeatLoop struct {
	trackerClient *TrackerClient
	interval      time.Duration
}

func NewHeartbeatLoop(trackerClient *TrackerClient) *HeartbeatLoop {
	return &HeartbeatLoop{trackerClient: trackerClient, interval: 30 * time.Second}
}

func (h *HeartbeatLoop) Start(ctx context.Context, idTask int64, onTaskGone func()) {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := h.trackerClient.SendTaskHeartbeat(ctx, idTask)
			if IsTrackerErrorCode(err, ErrCodeTaskNotRunningByAgent) {
				onTaskGone()
				return
			}
			if err != nil {
				log.Warn().Int64("idTask", idTask).Err(err).Msg("heartbeat failed")
			}
		}
	}
}
