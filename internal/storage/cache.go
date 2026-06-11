package storage

import "sync"

type StorageCache struct {
	mu   sync.RWMutex
	data map[string]Storage
}

func NewStorageCache() *StorageCache {
	return &StorageCache{
		data: make(map[string]Storage),
	}
}

func (this *StorageCache) Get(key string) (Storage, bool) {
	this.mu.RLock()
	defer this.mu.RUnlock()
	s, ok := this.data[key]
	return s, ok
}

func (this *StorageCache) Set(key string, storage Storage) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.data[key] = storage
}

func (this *StorageCache) IncreaseDownloadCount(key string) {
	this.update(key, func(s *Storage) *Storage {
		s.IncreaseDownloadCount()
		return s
	})
}

func (this *StorageCache) ChangeStatus(key string, status StorageFileStatus) {
	this.update(key, func(s *Storage) *Storage {
		s.SetStatus(status)
		return s
	})
}

func (this *StorageCache) update(key string, updateFn func(s *Storage) *Storage) {
	this.mu.Lock()
	defer this.mu.Unlock()

	old, ok := this.data[key]
	if !ok {
		return
	}
	new := updateFn(&old)
	this.data[key] = *new
}
