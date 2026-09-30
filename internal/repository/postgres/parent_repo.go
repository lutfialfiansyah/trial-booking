package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
)

// ParentRepository is the pgx-backed implementation of domain.ParentRepository.
type ParentRepository struct {
	pool *pgxpool.Pool
}

// NewParentRepository constructs a ParentRepository backed by the given pool.
func NewParentRepository(pool *pgxpool.Pool) *ParentRepository {
	return &ParentRepository{pool: pool}
}

// GetByID returns a single parent by id.
func (r *ParentRepository) GetByID(ctx context.Context, tx domain.Tx, id int64) (*domain.Parent, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT id, name, email, created_at
		FROM parents
		WHERE id = $1`

	var p domain.Parent
	err := db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.Name, &p.Email, &p.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrParentNotFound
		}
		return nil, fmt.Errorf("get parent by id: %w", err)
	}

	return &p, nil
}
