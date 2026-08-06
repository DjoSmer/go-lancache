package lancache_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/url"

	"go-lancache/internal/proxy"
	"go-lancache/internal/storage"
	"go-lancache/internal/target"
)

// MOCK FILE
type mockFile struct {
	buf          *bytes.Buffer
	closeCalled  bool
	removeCalled bool
}

func (m *mockFile) Write(p []byte) (n int, err error) { return m.buf.Write(p) }
func (m *mockFile) Read(p []byte) (n int, err error)  { return m.buf.Read(p) }
func (m *mockFile) Close() error                      { m.closeCalled = true; return nil }
func (m *mockFile) Remove() error                     { m.removeCalled = true; return nil }

// MOCK STORAGE SERVICE
type mockStorageService struct {
	lock          bool
	lockCalled    bool
	releaseCalled bool
	finalStatus   string
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
func (m *mockStorageService) CreateFile(storage *storage.Storage, write bool) storage.StorageFile {
	return m.fileMock
}
func (m *mockStorageService) Lock(storage *storage.Storage) (*storage.Storage, bool) {
	m.lockCalled = true
	return nil, m.lock
}
func (m *mockStorageService) MarkAsCorrupted(storage *storage.Storage) *storage.Storage {
	return nil
}
func (m *mockStorageService) Release(storage *storage.Storage, err error) *storage.Storage {
	m.releaseCalled = true
	return nil
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
	passFunc func(proxy *proxy.Proxy)
}

func (r *mockProxyRoute) CreateProxyContext(ctx context.Context, req *http.Request) *proxy.Proxy {
	text := "lancache_proxy_test"
	body := &mockFile{buf: new(bytes.Buffer)}
	body.Write([]byte(text))

	return &proxy.Proxy{
		Ctx:       context.Background(),
		RequestId: 1,
		Req:       &http.Request{URL: &url.URL{Path: "/game/patch.bin"}},
		Resp:      &http.Response{Body: body},
	}
}
func (r *mockProxyRoute) Pass(w http.ResponseWriter, proxy *proxy.Proxy) {
	if r.passFunc == nil {
		return
	}
	r.passFunc(proxy)
}
