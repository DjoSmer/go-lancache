package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrStorageNotFound = errors.New("storage not found")

type StorageRepository struct {
	db *sql.DB
}

func NewStorageRepository(db *sql.DB) *StorageRepository {
	return &StorageRepository{
		db: db,
	}
}

func (sr *StorageRepository) FindByBasePath(ctx context.Context, basePath string) (Storage, error) {
	var s Storage

	row := sr.db.QueryRowContext(ctx, `SELECT "basePath", "createdAt", "updatedAt", "downloadCount", "targetId", status FROM storage WHERE "basePath" = $1`, basePath)
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

func (sr *StorageRepository) Save(storage Storage) (Storage, error) {
	var err error

	if storage.IsNew {
		err = sr.add(&storage)
	} else {
		err = sr.update(&storage)
	}

	if err != nil {
		return storage, err
	}

	storage.IsNew = false
	return storage, nil
}

func (sr *StorageRepository) add(storage *Storage) error {
	const op = "storage.repo.add"

	stmt, err := sr.db.Prepare(`INSERT INTO storage ("basePath", "createdAt", "updatedAt", "downloadCount", "targetId", "status") VALUES ($1, $2, $3, $4, $5, $6)`)
	if err != nil {
		return fmt.Errorf("%s.prepare: %w", op, err)
	}

	_, err = stmt.Exec(storage.BasePath, storage.CreatedAt, storage.UpdatedAt, storage.DownloadCount, storage.TargetId, storage.Status)
	if err != nil {
		return fmt.Errorf("%s.exec: %w", op, err)
	}

	return nil
}

func (sr *StorageRepository) update(storage *Storage) error {
	const op = "storage.repo.update"

	stmt, err := sr.db.Prepare(`UPDATE storage SET "updatedAt" = $1, "downloadCount" = $2, status = $3 WHERE "basePath" = $4`)
	if err != nil {
		return fmt.Errorf("%s.prepare: %w", op, err)
	}

	_, err = stmt.Exec(storage.UpdatedAt, storage.DownloadCount, storage.Status, storage.BasePath)
	if err != nil {
		return fmt.Errorf("%s.exec: %w", op, err)
	}

	return nil
}
