package extctx

import (
	"context"

	"github.com/rs/zerolog"
)

// WithLogger attaches logger to ctx. Called by the logging middleware.
func WithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return logger.WithContext(ctx) // uses zerolog's own context key
}

// GetLogger retrieves the logger seeded by WithLogger.
// Falls back to the zerolog global if none was attached (e.g. in tests or startup code).
func GetLogger(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}
