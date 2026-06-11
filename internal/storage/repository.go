package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type StorageRepository struct {
	db *sql.DB
}

func NewStorageRepository(db *sql.DB) *StorageRepository {
	return &StorageRepository{
		db: db,
	}
}

func (sr *StorageRepository) Get(ctx context.Context, basePath string) (Storage, error) {
	var s Storage

	row := sr.db.QueryRowContext(ctx, "SELECT * FROM storage WHERE basePath = ?", basePath)
	err := row.Scan(&s.BasePath, &s.CreatedAt, &s.UpdatedAt, &s.DownloadCount, &s.TargetId, &s.Status)
	if err != nil {
		// if err == sql.ErrNoRows {
		// 	return s, fmt.Errorf("%s: %w", op, sql.ErrNoRows)
		// }
		return s, err
	}
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

	stmt, err := sr.db.Prepare(`INSERT INTO storage ("basePath", "createdAt", "updatedAt", "downloadCount", "targetId", status) VALUES (?, ?, ?, ?, ?, ?)`)
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

	stmt, err := sr.db.Prepare(`UPDATE storage SET "updatedAt" = ?, "downloadCount" = ?, status = ? WHERE "basePath" = ?`)
	if err != nil {
		return fmt.Errorf("%s.prepare: %w", op, err)
	}

	_, err = stmt.Exec(storage.UpdatedAt, storage.DownloadCount, storage.Status, storage.BasePath)
	if err != nil {
		return fmt.Errorf("%s.exec: %w", op, err)
	}

	return nil
}
