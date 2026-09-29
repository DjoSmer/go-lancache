package lancache_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go-lancache/internal/config"
	"go-lancache/internal/lancache"
	"go-lancache/internal/storage"
	"go-lancache/internal/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var globalMockStorage = &mockStorageService{
	lock: true,
	storage: &storage.Storage{
		BasePath: "game/dir/patch.bin",
		Status:   storage.StorageStatusSuccess,
	},
}
var globalMockTarget = &mockTargetService{
	getterCount: 0,
	target: &target.Target{
		Id:   1,
		Code: "game",
	},
}
var globalMockProxy = &mockProxyRoute{}
var globalAd = lancache.NewAnomalyDetector(config.AnomalyDetectorConfig{}, 1*time.Second, 3*time.Second)
var globalRoute = lancache.NewLancacheRoute(context.Background(), globalMockStorage, globalMockTarget, globalMockProxy, globalAd)

func TestLancacheRoute_Handler_NotGetMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	rec := httptest.NewRecorder()

	result := globalMockProxy.Handler(rec, req)
	assert.False(t, result)
	assert.Equal(t, globalMockTarget.getterCount, 0)
}

func TestLancacheRoute_Handler_Localhost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
	rec := httptest.NewRecorder()

	result := globalMockProxy.Handler(rec, req)
	assert.False(t, result)

	req = httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	rec = httptest.NewRecorder()
	result = globalMockProxy.Handler(rec, req)
	assert.False(t, result)
	assert.Equal(t, globalMockTarget.getterCount, 0)
}

func TestLancacheRoute_Handler_NotTarget(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/game/patch.bin", nil)
	rec := httptest.NewRecorder()

	var mockTarget = &mockTargetService{
		getterCount: 0,
		target:      nil,
	}

	route := lancache.NewLancacheRoute(context.Background(), globalMockStorage, mockTarget, globalMockProxy, globalAd)

	result := route.Handler(rec, req)
	assert.False(t, result)
	assert.Equal(t, mockTarget.getterCount, 1)
}

func TestLancacheRoute_Handler_ServeFileFromStorage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/dir/patch.bin", nil)
	rec := httptest.NewRecorder()

	nextGetterCount := globalMockTarget.getterCount + 1

	file, err := os.CreateTemp(t.TempDir(), "lancache-*")
	require.NoError(t, err)
	defer file.Close()

	text := "lancache_copy_test"
	wn, err := file.Write([]byte(text))

	mockStorage := &mockStorageService{
		fileMock: newMockFile(file),
		lock:     false,
		storage: &storage.Storage{
			BasePath: "game/dir/patch.bin",
			Status:   storage.StorageStatusSuccess,
		},
	}
	assert.False(t, mockStorage.lockCalled)

	route := lancache.NewLancacheRoute(context.Background(), mockStorage, globalMockTarget, globalMockProxy, globalAd)
	result := route.Handler(rec, req)
	assert.True(t, result)
	assert.Equal(t, globalMockTarget.getterCount, nextGetterCount)

	resp := rec.Result()
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("X-Lancache-Status"), "HIT")

	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, wn, len(data))
	assert.Equal(t, string(data), text)
}

func TestLancacheRoute_Handler_NotAvailable(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/dir/patch.bin", nil)
	rec := httptest.NewRecorder()

	mockStorage := &mockStorageService{
		storage: &storage.Storage{
			BasePath: "game/dir/patch.bin",
			Status:   storage.StorageStatusPending,
		},
	}

	mockProxy := &mockProxyRoute{
		handlerCallback: func(w http.ResponseWriter) {
			t.Fatal("Proxy.Handler must not be run!")
		},
	}

	route := lancache.NewLancacheRoute(context.Background(), mockStorage, globalMockTarget, mockProxy, globalAd)

	result := route.Handler(rec, req)
	assert.False(t, result)
	assert.False(t, mockStorage.lockCalled)

	resp := rec.Result()
	assert.Equal(t, resp.Header.Get("X-Lancache-Status"), "MISS")
}

func TestLancacheRoute_Handler_CannotLock(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/dir/patch.bin", nil)
	rec := httptest.NewRecorder()

	mockStorage := &mockStorageService{
		lock: false,
		storage: &storage.Storage{
			BasePath: "game/dir/patch.bin",
			Status:   storage.StorageStatusIdle,
		},
	}

	mockProxy := &mockProxyRoute{
		handlerCallback: func(w http.ResponseWriter) {
			t.Fatal("Proxy.Handler must not be run!")
		},
	}

	route := lancache.NewLancacheRoute(context.Background(), mockStorage, globalMockTarget, mockProxy, globalAd)

	result := route.Handler(rec, req)
	assert.False(t, result)
	assert.True(t, mockStorage.lockCalled)

	resp := rec.Result()
	assert.Equal(t, resp.Header.Get("X-Lancache-Status"), "MISS")
}

func TestLancacheRoute_Handler_UploadFullFileWithHeaderRange(t *testing.T) {
	text := "hello world world"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(text))
	}))
	defer srv.Close()

	rec := httptest.NewRecorder()

	file, err := os.CreateTemp(t.TempDir(), "lancache-*")
	require.NoError(t, err)
	defer file.Close()

	mockStorage := &mockStorageService{
		fileMock: newMockFile(file),
		lock:     true,
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	req.Header.Add("Range", "2-4")

	mockProxy := &mockProxyRoute{
		handlerCallback: func(w http.ResponseWriter) {
			t.Fatal("Proxy.Handler must not be run!")
		},
	}

	route := lancache.NewLancacheRoute(context.Background(), mockStorage, globalMockTarget, mockProxy, globalAd)
	result := route.Handler(rec, req)
	require.False(t, result)
	assert.True(t, mockStorage.lockCalled)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tinker := time.NewTicker(time.Second)
	defer tinker.Stop()

	for !mockStorage.releaseCalled {
		select {
		case <-tinker.C:
		case <-ctx.Done():
			t.Fatal("Timeout download")
		}
	}

	assert.True(t, mockStorage.fileMock.closeCalled)

	require.NotNil(t, mockStorage.storage)
	assert.Equal(t, mockStorage.storage.GetStatus(), storage.StorageStatusSuccess)

	file, err = os.Open(file.Name())
	assert.NoError(t, err)

	data, err := io.ReadAll(file)
	assert.NoError(t, err)
	fmt.Println(data)
	assert.Equal(t, string(data), text)
}

func TestLancacheRoute_Handler_Upload(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/dir/patch.bin", nil)
	rec := httptest.NewRecorder()

	file, err := os.CreateTemp(t.TempDir(), "lancache-*")
	require.NoError(t, err)
	defer file.Close()

	mockStorage := &mockStorageService{
		fileMock: newMockFile(file),
		lock:     true,
	}
	assert.False(t, mockStorage.lockCalled)

	text := "lancache_upload_test"
	mockProxy := &mockProxyRoute{
		handlerCallback: func(w http.ResponseWriter) {
			_, ok := w.(*lancache.LancacheResponseWriter)
			assert.True(t, ok)
			w.Write([]byte(text))
		},
	}

	route := lancache.NewLancacheRoute(context.Background(), mockStorage, globalMockTarget, mockProxy, globalAd)
	result := route.Handler(rec, req)
	assert.True(t, mockStorage.lockCalled)
	assert.True(t, mockStorage.releaseCalled)
	assert.True(t, mockStorage.fileMock.closeCalled)
	require.True(t, result)

	require.NotNil(t, mockStorage.storage)
	assert.Equal(t, mockStorage.storage.GetStatus(), storage.StorageStatusSuccess)

	resp := rec.Result()
	defer resp.Body.Close()
	assert.Equal(t, resp.Header.Get("X-Lancache-Status"), "MISS")

	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, string(data), text)

	file, err = os.Open(file.Name())
	assert.NoError(t, err)

	data, err = io.ReadAll(file)
	assert.NoError(t, err)
	assert.Equal(t, string(data), text)
}

func init() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	slog.SetDefault(log)
}
