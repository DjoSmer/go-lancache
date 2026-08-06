package lancache_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-lancache/internal/config"
	"go-lancache/internal/lancache"
	"go-lancache/internal/proxy"
	"go-lancache/internal/storage"
	"go-lancache/internal/target"

	"github.com/stretchr/testify/assert"
)

var mockStorage = &mockStorageService{
	fileMock: &mockFile{buf: new(bytes.Buffer)},
	lock:     true,
	storage: &storage.Storage{
		BasePath: "game/dir/patch.bin",
		Status:   storage.StorageStatusSuccess,
	},
}
var mockTarget = &mockTargetService{
	getterCount: 0,
	target: &target.Target{
		Id:   1,
		Code: "game",
	},
}
var mockProxy = &mockProxyRoute{}
var ad = lancache.NewAnomalyDetector(config.AnomalyDetectorConfig{}, 1*time.Second, 3*time.Second)
var route = lancache.NewLancacheRoute(mockStorage, mockTarget, mockProxy, ad)

func TestLancacheRoute_Handler_not_Get_Method(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://example.com", nil)
	rec := httptest.NewRecorder()

	result := route.Handler(rec, req)
	assert.False(t, result)
	assert.Equal(t, mockTarget.getterCount, 0)
}

func TestLancacheRoute_Handler_localhost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
	rec := httptest.NewRecorder()

	result := route.Handler(rec, req)
	assert.False(t, result)

	req = httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	rec = httptest.NewRecorder()
	result = route.Handler(rec, req)
	assert.False(t, result)
	assert.Equal(t, mockTarget.getterCount, 0)
}

func TestLancacheRoute_Handler_not_Target(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/game/patch.bin", nil)
	rec := httptest.NewRecorder()

	var mockTarget = &mockTargetService{
		getterCount: 0,
		target:      nil,
	}

	route := lancache.NewLancacheRoute(mockStorage, mockTarget, mockProxy, ad)

	result := route.Handler(rec, req)
	assert.False(t, result)
	assert.Equal(t, mockTarget.getterCount, 1)
}

func TestLancacheRoute_Handler_Storage_Download(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/dir/patch.bin", nil)
	rec := httptest.NewRecorder()

	nextGetterCount := mockTarget.getterCount + 1

	text := "lancache_copy_test"
	wn, err := mockStorage.fileMock.Write([]byte(text))

	result := route.Handler(rec, req)
	assert.True(t, result)
	assert.Equal(t, mockTarget.getterCount, nextGetterCount)

	resp := rec.Result()
	defer resp.Body.Close()

	assert.Equal(t, resp.Header.Get("X-Lancache-Status"), "HIT")

	data, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, wn, len(data))
	assert.Equal(t, string(data), text)
}

func TestLancacheRoute_Handler_Prepare_Upload(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/dir/patch.bin", nil)
	rec := httptest.NewRecorder()

	mockStorage := &mockStorageService{
		fileMock: &mockFile{buf: new(bytes.Buffer)},
		lock:     true,
		storage:  nil,
	}
	assert.False(t, mockStorage.lockCalled)

	mockProxy := &mockProxyRoute{
		passFunc: func(proxy *proxy.Proxy) {
			assert.NotNil(t, proxy.Hooks)
		},
	}

	route := lancache.NewLancacheRoute(mockStorage, mockTarget, mockProxy, ad)

	result := route.Handler(rec, req)
	assert.True(t, result)
	assert.True(t, mockStorage.lockCalled)

	resp := rec.Result()
	assert.Equal(t, resp.Header.Get("X-Lancache-Status"), "MISS")
}

func TestLancacheRoute_Handler_not_Available(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/dir/patch.bin", nil)
	rec := httptest.NewRecorder()

	mockStorage := &mockStorageService{
		storage: &storage.Storage{
			BasePath: "game/dir/patch.bin",
			Status:   storage.StorageStatusPending,
		},
	}

	mockProxy := &mockProxyRoute{
		passFunc: func(proxy *proxy.Proxy) {
			assert.Nil(t, proxy.Hooks)
		},
	}

	route := lancache.NewLancacheRoute(mockStorage, mockTarget, mockProxy, ad)

	result := route.Handler(rec, req)
	assert.True(t, result)
	assert.False(t, mockStorage.lockCalled)

	resp := rec.Result()
	assert.Equal(t, resp.Header.Get("X-Lancache-Status"), "MISS")
}

func TestLancacheRoute_Handler_cannot_Lock(t *testing.T) {
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
		passFunc: func(proxy *proxy.Proxy) {
			assert.Nil(t, proxy.Hooks)
		},
	}

	route := lancache.NewLancacheRoute(mockStorage, mockTarget, mockProxy, ad)

	result := route.Handler(rec, req)
	assert.True(t, result)
	assert.True(t, mockStorage.lockCalled)

	resp := rec.Result()
	assert.Equal(t, resp.Header.Get("X-Lancache-Status"), "MISS")
}
