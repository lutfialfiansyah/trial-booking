package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
)

// DBTX is the minimal database surface shared by *pgxpool.Pool and pgx.Tx.
// It lets every repository method run either inside a transaction or directly
// on the pool without knowing which one it is.
type DBTX interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// resolveDBTX returns the transaction when one is supplied, otherwise it falls
// back to the pool. Both pgx.Tx and *pgxpool.Pool satisfy DBTX.
func resolveDBTX(pool *pgxpool.Pool, tx domain.Tx) DBTX {
	if tx == nil {
		return pool
	}

	if pgxTx, ok := tx.(pgx.Tx); ok {
		return pgxTx
	}

	return pool
}
