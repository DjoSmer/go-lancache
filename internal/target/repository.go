package target

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrTargetNotFound = errors.New("target not found")

type TargetRepository struct {
	db *sql.DB
}

func NewTargetRepository(db *sql.DB) *TargetRepository {
	return &TargetRepository{db}
}

func (r *TargetRepository) FindById(ctx context.Context, id int) (Target, error) {
	var t Target

	row := r.db.QueryRowContext(ctx, `SELECT "id", "code", COALESCE(host, '') as host, COALESCE("userAgent", '') as "userAgent" FROM target WHERE id = ?`, id)
	err := row.Scan(&t.Id, &t.Code, &t.Domain, &t.UserAgent)
	if err != nil {
		if err == sql.ErrNoRows {
			return t, ErrTargetNotFound
		}
		return t, err
	}

	return t, nil
}

func (r *TargetRepository) GetAll(ctx context.Context) ([]Target, error) {
	var ts []Target

	rows, err := r.db.QueryContext(ctx, `SELECT "id", "code", COALESCE(host, '') as host, COALESCE("userAgent", '') as "userAgent" FROM target`)
	if err != nil {
		return nil, fmt.Errorf("target.getAll query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t Target

		err := rows.Scan(&t.Id, &t.Code, &t.Domain, &t.UserAgent)
		if err != nil {
			return nil, fmt.Errorf("target.getAll scan: %w", err)
		}

		ts = append(ts, t)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("target.getAll rows: %w", err)
	}

	return ts, nil
}

func (r *TargetRepository) Save(target Target) (Target, error) {
	panic("write me")
}
