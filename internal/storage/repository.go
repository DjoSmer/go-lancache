package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const op = "storage.repo"

var ErrStorageNotFound = errors.New("storage not found")

type StorageRepository struct {
	db *sql.DB
}

func NewStorageRepository(db *sql.DB) *StorageRepository {
	return &StorageRepository{
		db: db,
	}
}

func (r *StorageRepository) FindByBasePath(ctx context.Context, basePath string) (Storage, error) {
	var s Storage

	row := r.db.QueryRowContext(ctx, `SELECT "basePath", "createdAt", "updatedAt", "downloadCount", "targetId", status FROM storage WHERE "basePath" = $1`, basePath)
	err := row.Scan(&s.BasePath, &s.CreatedAt, &s.UpdatedAt, &s.DownloadCount, &s.TargetId, &s.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return s, ErrStorageNotFound
		}
		return s, err
	}

	s.IsNew = false
	return s, nil
}

func (r *StorageRepository) GetExpired(ctx context.Context, fileTTL time.Duration) ([]Storage, error) {
	storages := make([]Storage, 0, 100)

	rows, err := r.db.QueryContext(ctx, `SELECT "basePath" FROM storage where "updatedAt" <= $1`, time.Now().Truncate(fileTTL))
	if err != nil {
		return nil, fmt.Errorf("%s.getExpired query: %w", op, err)
	}
	defer rows.Close()

	for rows.Next() {
		var storage Storage

		err := rows.Scan(&storage.BasePath)
		if err != nil {
			return nil, fmt.Errorf("%s.getExpired scan: %w", op, err)
		}

		storages = append(storages, storage)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("%s.getExpired rows: %w", op, err)
	}

	return storages, nil
}

func (r *StorageRepository) GetOldest(ctx context.Context, limit int) ([]Storage, error) {
	storages := make([]Storage, 0, limit)

	rows, err := r.db.QueryContext(ctx, `SELECT "basePath" FROM storage Order by "updatedAt" ask Limit $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("%s.getOldest query: %w", op, err)
	}
	defer rows.Close()

	for rows.Next() {
		var storage Storage

		err := rows.Scan(&storage.BasePath)
		if err != nil {
			return nil, fmt.Errorf("%s.getOldest scan: %w", op, err)
		}

		storages = append(storages, storage)
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("%s.getOldest rows: %w", op, err)
	}

	return storages, nil
}

func (r *StorageRepository) DeleteMany(ctx context.Context, storages []Storage) error {
	storagesLen := len(storages)
	if storagesLen == 0 {
		return nil
	}

	args := make([]any, storagesLen)
	placeholders := make([]string, storagesLen)
	for i, storage := range storages {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = storage.BasePath
	}

	query := fmt.Sprintf(`DELETE FROM storage WHERE "basePath" IN (%s)`, strings.Join(placeholders, ", "))

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s.delete.exec: %w", op, err)
	}
	return nil
}

func (r *StorageRepository) Save(ctx context.Context, storage Storage) (Storage, error) {
	var err error

	if storage.IsNew {
		err = r.add(ctx, &storage)
	} else {
		err = r.update(ctx, &storage)
	}

	if err != nil {
		return storage, err
	}

	storage.IsNew = false
	return storage, nil
}

func (r *StorageRepository) add(ctx context.Context, storage *Storage) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO storage ("basePath", "createdAt", "updatedAt", "downloadCount", "targetId", "status") 
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT ("basePath") 
		DO UPDATE SET "updatedAt" = $3, "status" = $6`,
		storage.BasePath, storage.CreatedAt, storage.UpdatedAt, storage.DownloadCount, storage.TargetId, storage.Status,
	)
	if err != nil {
		return fmt.Errorf("%s.add: %w", op, err)
	}

	return nil
}

func (r *StorageRepository) update(ctx context.Context, storage *Storage) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE storage SET "updatedAt" = $1, "downloadCount" = $2, status = $3 WHERE "basePath" = $4`,
		storage.UpdatedAt, storage.DownloadCount, storage.Status, storage.BasePath,
	)
	if err != nil {
		return fmt.Errorf("%s.update.prepare: %w", op, err)
	}

	return nil
}
