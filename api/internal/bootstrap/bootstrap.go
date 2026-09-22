package bootstrap

import (
	"context"
	"log"

	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/spf13/viper"
)

func configure() {
	viper.AutomaticEnv()
	viper.SetDefault("PROJECT_BUILDER_DESCRIPTION_MAX_LENGTH", 10000)
	viper.SetDefault("WEBSOCKET_WRITE_DEADLINE", "10s")
	viper.SetDefault("MERGE_POLL_INTERVAL", constants.DefaultMergePollInterval)
}

// Run boots the application and serves until ctx is cancelled.
func Run(ctx context.Context) error {
	configure()

	app, err := issue.New()
	if err != nil {
		return err
	}

	if err := injector.GetSkillService().SyncBuiltins(ctx); err != nil {
		return err
	}

	injector.GetIssueService().StartIdempotencyCleanup(ctx)

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
	return err
}
