package storage

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"
)

type Config struct {
	StoragePath string
}

type StorageService struct {
	config Config
	repo   *StorageRepository
	cache  *StorageCache
}

func NewStorageService(repo *StorageRepository) *StorageService {
	return &StorageService{
		cache: NewStorageCache(),
		repo:  repo,
	}
}

func (ss *StorageService) Create(basePath string, targetId uint16) *Storage {
	log := ss.getLog("Create")

	storage := &Storage{
		BasePath:      basePath,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		DownloadCount: 0,
		TargetId:      targetId,
		IsNew:         true,
	}

	log.Info("Create new storage: " + basePath)
	ss.cache.Set(basePath, *storage)
	return storage
}

func (ss *StorageService) Get(ctx context.Context, basePath string) (*Storage, error) {
	log := ss.getLog("Create")
	storage, ok := ss.cache.Get(basePath)
	if ok {
		return &storage, nil
	}

	storage, err := ss.repo.Get(ctx, basePath)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Error("Storage repo return error: " + err.Error())
		}
		return nil, err
	}

	ss.cache.Set(basePath, storage)
	return &storage, nil
}

func (ss *StorageService) CreateFile(storage *Storage, write bool) *StorageFile {
	return NewStorageFile(storage, ss.config.StoragePath, write)
}

func (ss *StorageService) IncreaseDownloadCount(storage *Storage) *Storage {
	return ss.cache.Update(storage.BasePath, func(storage *Storage) *Storage {
		storage.IncreaseDownloadCount()
		return storage
	})
}

func (ss *StorageService) ChangeStatus(storage *Storage, status StorageStatus) *Storage {
	return ss.cache.Update(storage.BasePath, func(storage *Storage) *Storage {
		storage.SetStatus(status)
		return storage
	})
}

func (ss *StorageService) Save(s Storage) {
	s, err := ss.repo.Save(s)
	if err != nil {

	}

	ss.cache.Set(s.BasePath, s)

	/*
				const storageFile = this.openStorageFiles.get(data.basePath);
		    if (storageFile && storageFile.instances.size < 1) this.close(data);
	*/
}

func (ss *StorageService) getLog(name string) *slog.Logger {
	op := "StorageService." + name
	return slog.Default().With(slog.String("op", op))
}
