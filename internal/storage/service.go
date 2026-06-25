package storage

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"go-lancache/internal/config"
)

type StorageService struct {
	config config.StorageConfig
	repo   *StorageRepository
	cache  *StorageCache
}

func NewStorageService(repo *StorageRepository, cache *StorageCache, config config.StorageConfig) *StorageService {
	return &StorageService{
		cache:  cache,
		repo:   repo,
		config: config,
	}
}

func (ss *StorageService) Create(basePath string, targetId int) *Storage {
	log := ss.log("Create")

	storage := &Storage{
		BasePath:      basePath,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		DownloadCount: 0,
		TargetId:      targetId,
		IsNew:         true,
		Status:        StorageStatusIdle,
	}

	log.Debug("Create new storage: " + basePath)
	ss.cache.Set(basePath, *storage)
	return storage
}

func (ss *StorageService) Get(ctx context.Context, basePath string) (*Storage, error) {
	log := ss.log("Get")

	storage, ok := ss.cache.Get(basePath)
	if ok {
		log.Debug("Get storage from cache: " + basePath)
		return &storage, nil
	}

	storage, err := ss.repo.FindByBasePath(ctx, basePath)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Error("Storage repo return error: " + err.Error())
		}
		return nil, err
	}

	log.Debug("Get storage from repo: " + basePath)
	ss.cache.Set(basePath, storage)
	return &storage, nil
}

func (ss *StorageService) CreateFile(storage *Storage, write bool) StorageFile {
	return NewStorageFile(storage, ss.config.Path, write)
}

func (ss *StorageService) IncreaseDownloadCount(storage *Storage) *Storage {
	return ss.cache.Update(storage.BasePath, func(storage *Storage) *Storage {
		storage.IncreaseDownloadCount()
		return storage
	})
}

func (ss *StorageService) Lock(storage *Storage) (*Storage, bool) {
	result := true
	storage = ss.cache.Update(storage.BasePath, func(storage *Storage) *Storage {
		if storage.IsAvailable() {
			storage.SetStatus(StorageStatusPending)
			return storage
		}
		result = false
		storage.SetStatus(StorageStatusSkip)
		return nil
	})

	return storage, result
}

func (ss *StorageService) Release(storage *Storage, err error) *Storage {
	storage = ss.cache.Update(storage.BasePath, func(storage *Storage) *Storage {
		if err == nil {
			storage.SetStatus(StorageStatusSuccess)
		} else {
			storage.SetStatus(StorageStatusError)
		}
		return storage
	})

	ss.Save(*storage)
	return storage
}

func (ss *StorageService) Save(storage Storage) {
	log := ss.log("Save")

	storage, err := ss.repo.Save(storage)
	if err != nil {
		log.Error("Cannot save storage: " + err.Error())
		return
	}

	ss.cache.Set(storage.BasePath, storage)
}

func (ss *StorageService) log(name string) *slog.Logger {
	op := "StorageService." + name
	return slog.Default().With(slog.String("op", op))
}
