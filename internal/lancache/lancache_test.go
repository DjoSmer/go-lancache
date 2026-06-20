package lancache_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"go-lancache/internal/lancache"
	"go-lancache/internal/proxy"
	"go-lancache/internal/storage"

	"github.com/stretchr/testify/assert"
)

var proxyRoute = proxy.NewProxyRoute()

func TestLancache_InterceptResponse(t *testing.T) {
	mockStorage := &mockStorageService{}
	storage := &storage.Storage{}
	lancache := lancache.NewLancache(storage, mockStorage)

	proxy := &proxy.Proxy{
		Ctx:       context.Background(),
		RequestId: 1,
		Req:       &http.Request{URL: &url.URL{Path: "/game/patch.bin"}},
		Resp:      &http.Response{},
		Hooks:     lancache,
	}

	orgBody := proxy.Resp.Body
	assert.Equal(t, proxy.Resp.Body, orgBody)

	lancache.InterceptResponse(proxy)
	assert.NotEqual(t, proxy.Resp.Body, orgBody)
}

func TestLancache_Read(t *testing.T) {
	mockStorage := &mockStorageService{
		fileMock: &mockFile{buf: new(bytes.Buffer)},
	}
	storage := &storage.Storage{}
	lancache := lancache.NewLancache(storage, mockStorage)

	text := "lancache_rw_test"
	body := &mockFile{buf: new(bytes.Buffer)}
	wn, err := body.Write([]byte(text))

	proxy := &proxy.Proxy{
		Ctx:       context.Background(),
		RequestId: 1,
		Req:       &http.Request{URL: &url.URL{Path: "/game/patch.bin"}},
		Resp:      &http.Response{Body: body},
		Hooks:     lancache,
	}

	lancache.InterceptResponse(proxy)

	result, err := io.ReadAll(proxy.Resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, wn, len(result))
	assert.Equal(t, string(result), text)
}

func TestLancache_Close(t *testing.T) {
	mockStorage := &mockStorageService{}
	storage := &storage.Storage{}
	lancache := lancache.NewLancache(storage, mockStorage)

	fileMock := &mockFile{buf: new(bytes.Buffer)}
	lancache.StorageFile = fileMock

	lancache.Close()
	assert.True(t, fileMock.closeCalled)
}

func TestLancache_AfterResponse(t *testing.T) {
	mockStorage := &mockStorageService{}
	storage := &storage.Storage{}
	lancache := lancache.NewLancache(storage, mockStorage)

	fileMock := &mockFile{buf: new(bytes.Buffer)}
	lancache.StorageFile = fileMock

	lancache.AfterResponse(nil, nil)

	assert.False(t, fileMock.closeCalled)
	assert.False(t, fileMock.removeCalled)
	assert.True(t, mockStorage.releaseCalled)
}

func TestLancache_AfterResponse_HandlesError(t *testing.T) {
	mockStorage := &mockStorageService{}
	storage := &storage.Storage{}
	lancache := lancache.NewLancache(storage, mockStorage)

	fileMock := &mockFile{buf: new(bytes.Buffer)}
	lancache.StorageFile = fileMock

	simulatedError := errors.New("network timeout")
	lancache.AfterResponse(nil, simulatedError)

	assert.True(t, fileMock.closeCalled)
	assert.True(t, fileMock.removeCalled)
	assert.True(t, mockStorage.releaseCalled)
}
