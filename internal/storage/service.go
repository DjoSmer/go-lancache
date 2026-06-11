package storage

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path"
	"syscall"
	"time"
)

type Config struct {
	StoragePath string
}

type StorageService struct {
	log    *slog.Logger
	config Config
	repo   *StorageRepository
	cache  *StorageCache
}

func NewStorageService(repo *StorageRepository, cache *StorageCache, log *slog.Logger) *StorageService {
	return &StorageService{
		cache: cache,
		log:   log,
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

// Return fullPath, or empty string if there has a problem
func (ss *StorageService) GetFilepath(s *Storage) string {
	log := ss.getLog("GetFilepath").With(slog.String("bp", s.BasePath))

	basePath := s.BasePath
	ext := path.Ext(basePath)

	if ext == "" {
		basePath = basePath + ".file"
	}

	filepath := path.Join(ss.config.StoragePath, basePath)

	if s.IsNew == false {
		return filepath
	}

	dir := path.Dir(filepath)
	err := os.MkdirAll(dir, 0775)
	if errors.Is(err, syscall.ENOTDIR) {
		log.Warn("Is not dir: " + dir)
		err := os.Remove(dir)
		if err != nil {
			log.Warn("Cannot remove dir: " + dir)
			return ""
		}
		err = os.MkdirAll(dir, os.ModeDir)
		if err != nil {
			log.Warn("Cannot mkdir dir: " + dir)
			return ""
		}
	}

	return filepath
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
	return ss.log.With(slog.String("op", op))
}
