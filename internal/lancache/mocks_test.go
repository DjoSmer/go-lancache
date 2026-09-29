package lancache_test

import (
	"context"
	"errors"
	"net/http"
	"os"

	"go-lancache/internal/storage"
	"go-lancache/internal/target"
)

// MOCK FILE
type mockFile struct {
	*os.File
	closeCalled  bool
	removeCalled bool
}

func newMockFile(file *os.File) *mockFile {
	return &mockFile{
		File: file,
	}
}

func (m *mockFile) Close() error {
	m.closeCalled = true
	return m.File.Close()
}
func (m *mockFile) Remove() error {
	m.removeCalled = true
	return os.Remove(m.Name())
}

// MOCK STORAGE SERVICE
type mockStorageService struct {
	lock          bool
	lockCalled    bool
	releaseCalled bool
	storage       *storage.Storage
	fileMock      *mockFile
}

func (m *mockStorageService) Get(ctx context.Context, key string) (*storage.Storage, error) {
	if m.storage == nil {
		return nil, errors.New("Storage not found")
	}
	return m.storage, nil
}

func (m *mockStorageService) Create(key string, targetId int) *storage.Storage {
	return &storage.Storage{
		BasePath: "game/dir/patch.bin",
		Status:   storage.StorageStatusIdle,
		IsNew:    true,
	}
}

func (m *mockStorageService) IncreaseDownloadCount(storage *storage.Storage) *storage.Storage {
	return storage
}
func (m *mockStorageService) GetFile(storage *storage.Storage) (storage.StorageFile, error) {
	return m.fileMock, nil
}
func (m *mockStorageService) Lock(storage *storage.Storage) (*storage.Storage, bool) {
	m.lockCalled = true
	return storage, m.lock
}
func (m *mockStorageService) MarkAsCorrupted(storage *storage.Storage) *storage.Storage {
	return storage
}
func (m *mockStorageService) Release(ctx context.Context, s *storage.Storage, err error) *storage.Storage {
	m.releaseCalled = true
	if err != nil {
		s.Status = storage.StorageStatusError
	} else {
		s.Status = storage.StorageStatusSuccess
	}
	m.storage = s
	return m.storage
}

// MOCK TARGET SERVICE
type mockTargetService struct {
	getterCount int
	target      *target.Target
}

func (s *mockTargetService) Get(cxt context.Context, host string, userAgent string, url string) (target.Target, error) {
	s.getterCount++
	if s.target == nil {
		return target.Target{}, errors.New("Target not found")
	}
	return *s.target, nil
}

// MOCK PROXY ROUTE
type mockProxyRoute struct {
	handlerCallback func(w http.ResponseWriter)
}

func (r *mockProxyRoute) Handler(w http.ResponseWriter, req *http.Request) bool {
	if r.handlerCallback == nil {
		return false
	}
	r.handlerCallback(w)
	return true
}
