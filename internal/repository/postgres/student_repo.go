package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"trial-booking/internal/domain"
)

// StudentRepository is the pgx-backed implementation of domain.StudentRepository.
type StudentRepository struct {
	pool *pgxpool.Pool
}

// NewStudentRepository constructs a StudentRepository backed by the given pool.
func NewStudentRepository(pool *pgxpool.Pool) *StudentRepository {
	return &StudentRepository{pool: pool}
}

// GetByID returns a single student by id.
func (r *StudentRepository) GetByID(ctx context.Context, tx domain.Tx, id int64) (*domain.Student, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT id, parent_id, name, created_at
		FROM students
		WHERE id = $1`

	var s domain.Student
	err := db.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.ParentID, &s.Name, &s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrStudentNotFound
		}
		return nil, fmt.Errorf("get student by id: %w", err)
	}

	return &s, nil
}

// GetByParentID returns all students belonging to a parent.
func (r *StudentRepository) GetByParentID(ctx context.Context, tx domain.Tx, parentID int64) ([]domain.Student, error) {
	db := resolveDBTX(r.pool, tx)

	const query = `
		SELECT id, parent_id, name, created_at
		FROM students
		WHERE parent_id = $1
		ORDER BY id ASC`

	rows, err := db.Query(ctx, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("list students by parent: %w", err)
	}
	defer rows.Close()

	students := make([]domain.Student, 0)
	for rows.Next() {
		var s domain.Student
		if err := rows.Scan(&s.ID, &s.ParentID, &s.Name, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan student: %w", err)
		}
		students = append(students, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate students: %w", err)
	}

	return students, nil
}
