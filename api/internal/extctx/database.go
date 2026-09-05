package extctx

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Interface satisfied by both pgx.Tx and pgxpool.Pool.
type PersistentStorage interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type txKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func GetDb(ctx context.Context, pool *pgxpool.Pool) PersistentStorage {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

type afterCommitKey struct{}

type afterCommitHooks struct {
	mu  sync.Mutex
	fns []func(context.Context)
}

func (h *afterCommitHooks) add(fn func(context.Context)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fns = append(h.fns, fn)
}

func (h *afterCommitHooks) run(ctx context.Context) {
	h.mu.Lock()
	fns := h.fns
	h.fns = nil
	h.mu.Unlock()
	for _, fn := range fns {
		fn(ctx)
	}
}

// AfterCommit defers fn until the ambient transaction commits, or runs it immediately when there is none.
// fn receives a detached context: the transaction and the request cancellation are gone, so it can read on the pool.
func AfterCommit(ctx context.Context, fn func(context.Context)) {
	hooks, ok := ctx.Value(afterCommitKey{}).(*afterCommitHooks)
	if !ok {
		fn(detach(ctx))
		return
	}
	hooks.add(fn)
}

func detach(ctx context.Context) context.Context {
	ctx = context.WithoutCancel(ctx)
	ctx = context.WithValue(ctx, txKey{}, nil)
	return context.WithValue(ctx, afterCommitKey{}, nil)
}

func RunInTx(ctx context.Context, pool *pgxpool.Pool, fn func(context.Context) error) (err error) {
	log := GetLogger(ctx)

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("RunInTx: failed to begin transaction")
		return err
	}
	hooks := &afterCommitHooks{}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	err = fn(context.WithValue(WithTx(ctx, tx), afterCommitKey{}, hooks))
	if err != nil {
		log.Error().Err(err).Msg("RunInTx: transaction function failed")
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("RunInTx: failed to commit transaction")
		return err
	}
	hooks.run(detach(ctx))
	return nil
}
