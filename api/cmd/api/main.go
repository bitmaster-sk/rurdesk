package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/spf13/viper"
)

// configure wires viper to env vars and sets the defaults the app relies on when
// a var is unset. Called explicitly at startup (over an init()) so ordering is obvious.
func configure() {
	viper.AutomaticEnv()
	viper.SetDefault("PROJECT_BUILDER_DESCRIPTION_MAX_LENGTH", 10000)
	viper.SetDefault("WEBSOCKET_WRITE_DEADLINE", "10s")
}

func main() {
	configure()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := issue.New()
	if err != nil {
		log.Fatal(err)
	}

	if err := injector.GetSkillService().SyncBuiltins(ctx); err != nil {
		log.Fatal(err)
	}

	injector.GetIssueService().StartIdempotencyCleanup(ctx)
	// MCP is no longer a separate listener — it is mounted on the same engine
	// (/mcp) and served by app.Start below.

	sweep := injector.GetSweep()
	if err := sweep.RunCrashRecovery(ctx); err != nil {
		log.Printf("crash recovery error: %v", err)
	}
	go sweep.StartHeartbeatSweep(ctx)
	go injector.GetMergePoller().Start(ctx)
	go injector.GetScheduler().Start(ctx)
	go injector.GetJobScheduler().Start(ctx)

	err = app.Start(ctx)
	injector.GetApiKeyService().Shutdown()
	if err != nil {
		log.Fatal(err)
	}
}
