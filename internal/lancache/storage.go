package lancache

import (
	"context"

	"go-lancache/internal/storage"
)

type storageService interface {
	Get(ctx context.Context, key string) (*storage.Storage, error)
	Create(key string, targetId int) *storage.Storage
	CreateFile(storage *storage.Storage, write bool) storage.StorageFile
	Lock(storage *storage.Storage) (*storage.Storage, bool)
	MarkAsCorrupted(storage *storage.Storage) *storage.Storage
	Release(storage *storage.Storage, err error) *storage.Storage
	IncreaseDownloadCount(storage *storage.Storage) *storage.Storage
}
