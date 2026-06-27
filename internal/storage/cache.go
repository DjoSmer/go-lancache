package storage

import (
	"log/slog"
	"sync"
)

type StorageCache struct {
	mu   sync.RWMutex
	data map[string]Storage
}

func NewStorageCache() *StorageCache {
	return &StorageCache{
		data: make(map[string]Storage),
	}
}

func (c *StorageCache) Get(key string) (Storage, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, ok := c.data[key]
	return s, ok
}

func (c *StorageCache) Set(key string, storage Storage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = storage
}

func (c *StorageCache) Update(key string, updateFn func(s *Storage) *Storage) *Storage {
	log := c.log("Update")

	c.mu.Lock()
	defer c.mu.Unlock()

	storage, ok := c.data[key]
	if !ok {
		log.Debug("Not found in cache")
		return nil
	}

	orgStorage := storage

	result := updateFn(&storage)
	if result != nil {
		log.Debug("Updated", slog.Any("org", orgStorage), slog.Any("new", result))
		c.data[key] = *result
		return result
	}

	return &storage
}

func (c *StorageCache) log(name string) *slog.Logger {
	op := "StorageCache." + name
	return slog.Default().With(slog.String("op", op))
}
