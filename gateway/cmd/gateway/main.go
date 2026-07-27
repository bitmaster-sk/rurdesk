package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	claudecode "github.com/bitmaster-sk/rurdesk/gateway/claude-code"
	"github.com/bitmaster-sk/rurdesk/gateway/common"
	"github.com/bitmaster-sk/rurdesk/gateway/goose"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	adapter := flag.String("adapter", "", "adapter type: claude-code | goose")
	flag.Parse()

	if *adapter == "" {
		log.Fatal().Msg("--adapter flag is required (claude-code | goose)")
	}

	cfg, err := common.LoadConfig(*adapter)
	if err != nil {
		log.Fatal().Err(err).Msg("config error")
	}

	setupLogger(cfg.LogLevel)

	agentAdapter := selectAdapter(cfg)
	dedup := common.NewDedupCache(24 * time.Hour)
	state := common.NewState()
	trackerClient := common.NewTrackerClient(cfg)
	orchestrator := common.NewOrchestrator(cfg, agentAdapter, trackerClient, state)

	if err := common.CloneRepo(cfg); err != nil {
		log.Fatal().Err(err).Msg("failed to clone repo")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Tells the tracker we (re)started so it fails our orphaned tasks (their
	// subprocesses died with us). Retried in the background (capped backoff) so
	// a tracker not yet up at gateway start still gets the signal promptly,
	// instead of waiting ~10min for the heartbeat sweep. GatewayRecovered is
	// idempotent except in one window: if the first call succeeded but its
	// response was lost, the retry's FailActiveForBot could fail a task
	// dispatched since then. Accepted for now. Stops on first success, on
	// shutdown, or after the attempt cap (heartbeat sweep is the backstop);
	// never blocks server start.
	go reportRecoveredWithRetry(ctx, trackerClient)

	// Crash recovery is API-side (a sweep fails stale tasks); the gateway only
	// runs a retention cron pruning worktrees idle past the retention window.
	go common.NewRetention(cfg, orchestrator).Start(ctx)

	server := common.NewServer(cfg, orchestrator, dedup)
	log.Info().Str("adapter", *adapter).Int("port", cfg.ListenPort).Msg("gateway starting")
	if err := server.Serve(ctx); err != nil {
		log.Error().Err(err).Msg("gateway stopped with an error")
	}
}

// reportRecoveredWithRetry reports gateway recovery to the tracker with capped
// exponential backoff until success, ctx cancellation, or the attempt cap (see
// the call site for why retrying is safe). The heartbeat sweep is the backstop.
func reportRecoveredWithRetry(ctx context.Context, tc *common.TrackerClient) {
	const (
		maxAttempts = 12
		baseDelay   = 1 * time.Second
		maxDelay    = 30 * time.Second
	)
	delay := baseDelay
	for attempt := 1; ; attempt++ {
		if err := tc.ReportRecovered(ctx); err == nil {
			log.Info().Msg("gateway recovery reported")
			return
		} else {
			log.Warn().Err(err).Int("attempt", attempt).Msg("gateway recovery report failed; will retry")
		}
		if attempt >= maxAttempts {
			log.Warn().Msg("gateway recovery report gave up; heartbeat sweep will reap orphaned tasks")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < maxDelay {
			if delay *= 2; delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}

func setupLogger(level string) {
	var l zerolog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = zerolog.DebugLevel
	case "warn", "warning":
		l = zerolog.WarnLevel
	case "error":
		l = zerolog.ErrorLevel
	default:
		l = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(l)
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	log.Info().Str("requestedLevel", level).Str("effectiveLevel", l.String()).Msg("logger configured")
}

func selectAdapter(cfg *common.Config) common.Agent {
	switch cfg.AdapterType {
	case "claude-code":
		return claudecode.NewClaudeCodeAdapter(cfg)
	case "goose":
		return goose.NewGooseAdapter(cfg)
	default:
		log.Fatal().Str("adapter", cfg.AdapterType).Msg("unknown adapter type")
		return nil
	}
}
