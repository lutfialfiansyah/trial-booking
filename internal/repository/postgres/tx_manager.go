package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
)

// TxManager runs domain operations inside a database transaction.
// It satisfies domain.TxManager.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager constructs a TxManager backed by the given pool.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithTx begins a transaction, invokes fn with the transaction (exposed to the
// domain as domain.Tx), and commits or rolls back based on the result.
func (m *TxManager) WithTx(ctx context.Context, fn func(tx domain.Tx) error) error {
	pgxTx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err := fn(pgxTx); err != nil {
		if rbErr := pgxTx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("tx error: %w; rollback error: %v", err, rbErr)
		}
		return err
	}

	if err := pgxTx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
