package target

import (
	"context"
	"database/sql"
	"fmt"
)

type DBTargetRepository struct {
	db *sql.DB
}

func NewDBTargetRepository(db *sql.DB) *DBTargetRepository {
	return &DBTargetRepository{db}
}

func (r *DBTargetRepository) FindById(ctx context.Context, id int) (Target, error) {
	var t Target

	row := r.db.QueryRowContext(ctx, `SELECT "id", "code", "host", "userAgent" FROM target WHERE id = ?`, id)
	err := row.Scan(&t.Id, &t.Code, &t.Domain, &t.UserAgent)
	if err != nil {
		if err == sql.ErrNoRows {
			return t, ErrTargetNotFound
		}
		return t, err
	}

	return t, nil
}

func (r *DBTargetRepository) GetAll(ctx context.Context) ([]Target, error) {
	var ts []Target

	rows, err := r.db.QueryContext(ctx, `SELECT "id", "code", "host", "userAgent" FROM target`)
	if err != nil {
		return nil, fmt.Errorf("target.getAll: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t Target

		err := rows.Scan(&t.Id, &t.Code, &t.Domain, &t.UserAgent)
		if err != nil {
			return nil, fmt.Errorf("target.getAll: %v", err)
		}

		ts = append(ts, t)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("target.getAll: %v", err)
	}

	return ts, nil
}

func (r *DBTargetRepository) Save(target Target) (Target, error) {
	panic("write me")
}
