package extctx

import (
	"context"

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

func RunInTx(ctx context.Context, pool *pgxpool.Pool, fn func(context.Context) error) (err error) {
	log := GetLogger(ctx)

	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Error().Err(err).Msg("RunInTx: failed to begin transaction")
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	err = fn(WithTx(ctx, tx))
	if err != nil {
		log.Error().Err(err).Msg("RunInTx: transaction function failed")
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		log.Error().Err(err).Msg("RunInTx: failed to commit transaction")
	}
	return err
}
