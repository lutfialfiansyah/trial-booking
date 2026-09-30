package domain

import "context"

// Tx is a transaction abstraction. The domain layer does not know
// about pgx. The postgres repository will type-assert this to pgx.Tx.
type Tx any

// TxManager runs a function inside a database transaction.
type TxManager interface {
	WithTx(ctx context.Context, fn func(tx Tx) error) error
}
