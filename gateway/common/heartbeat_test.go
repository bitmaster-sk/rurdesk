package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newHeartbeatTracker(t *testing.T, status int, body string, calls *atomic.Int32) *TrackerClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return NewTrackerClient(&Config{TrackerAPIUrl: server.URL, WebhookSecret: []byte("secret")})
}

func TestHeartbeatLoopStopsTheTaskOnlyWhenTheTrackerNoLongerRunsIt(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		wantTaskGone bool
	}{
		{"task not running by agent", http.StatusConflict, `{"code":"TASK_NOT_RUNNING_BY_AGENT"}`, true},
		{"other conflict", http.StatusConflict, `{"code":"SOMETHING_ELSE"}`, false},
		{"server error", http.StatusInternalServerError, ``, false},
		{"ok", http.StatusOK, `{"ok":true}`, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			loop := &HeartbeatLoop{
				trackerClient: newHeartbeatTracker(t, test.status, test.body, &calls),
				interval:      10 * time.Millisecond,
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			taskGone := make(chan struct{})
			loopDone := make(chan struct{})
			go func() {
				loop.Start(ctx, 7, func() { close(taskGone) })
				close(loopDone)
			}()

			select {
			case <-taskGone:
				if !test.wantTaskGone {
					t.Fatal("task was stopped, want it kept running")
				}
				select {
				case <-loopDone:
				case <-time.After(time.Second):
					t.Fatal("heartbeat loop kept running after the task was stopped")
				}
			case <-time.After(300 * time.Millisecond):
				if test.wantTaskGone {
					t.Fatal("task was not stopped within 300ms")
				}
				if calls.Load() < 2 {
					t.Fatalf("heartbeat sent %d times, want it to keep sending", calls.Load())
				}
			}
		})
	}
}
