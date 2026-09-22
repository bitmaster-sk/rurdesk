package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/bitmaster-sk/rurdesk/api/internal/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
