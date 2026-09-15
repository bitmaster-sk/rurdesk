package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/commercial/licensekey"
	"github.com/bitmaster-sk/rurdesk/api/internal/bootstrap"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/spf13/viper"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	installLicense()

	if err := bootstrap.Run(ctx); err != nil {
		log.Fatal(err)
	}
}

// installLicense never aborts the boot: a missing or broken key leaves paid
// features off instead of locking the owner out of their data.
func installLicense() {
	viper.AutomaticEnv()

	provider, err := licensekey.FromEnv()
	switch {
	case errors.Is(err, licensekey.ErrNotConfigured):
		log.Print("license: no key configured, paid features are off")
	case err != nil:
		log.Printf("license: key rejected (%v), paid features are off", err)
	default:
		status := provider.Status(context.Background())
		log.Printf("license: valid for %q until %s", status.Tenant, status.ExpiresAt.Format(time.RFC3339))
		injector.SetLicenseProvider(provider)
	}
}
