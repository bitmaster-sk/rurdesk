package issue

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/router"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"
)

// shutdownTimeout bounds how long in-flight requests have to drain on SIGTERM
// before the server is force-closed. SSE/WebSocket streams are long-lived, so
// this is a backstop — the orchestrator/LB is expected to stop routing new
// traffic before sending the signal.
const shutdownTimeout = 15 * time.Second

type Application struct {
	engine *gin.Engine
	Router *router.Router
	Pool   *pgxpool.Pool
	Cache  *redis.Client
}

func New() (*Application, error) {
	pool, err := injector.GetDb()
	if err != nil {
		return nil, err
	}
	routerInstance, err := injector.GetRouter()
	if err != nil {
		return nil, err
	}
	cache, err := injector.GetCache()
	if err != nil {
		return nil, err
	}
	engine := injector.GetHttpServer()
	return &Application{engine: engine, Router: routerInstance, Pool: pool, Cache: cache}, nil
}

func (a *Application) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.engine.ServeHTTP(w, r)
}

// Start runs the HTTP server until ctx is cancelled (SIGINT/SIGTERM), then
// gracefully drains in-flight requests within shutdownTimeout. The server
// carries REST, WebSocket, the mounted MCP handler and the SPA on one port.
func (a *Application) Start(ctx context.Context) error {
	addr := net.JoinHostPort(viper.GetString("APPLICATION_HOST"), viper.GetString("APPLICATION_PORT"))
	srv := &http.Server{Addr: addr, Handler: a.engine}

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
