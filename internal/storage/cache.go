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
	_, ok := this.data[key]
	if ok {
		return
	}
	this.data[key] = storage
}

func (this *StorageCache) Update(key string, updateFn func(s *Storage) *Storage) *Storage {
	this.mu.Lock()
	defer this.mu.Unlock()

	old, ok := this.data[key]
	if !ok {
		return nil
	}
	new := updateFn(&old)
	this.data[key] = *new
	return new
}
