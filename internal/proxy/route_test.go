package proxy_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"go-lancache/internal/proxy"
)

var proxyRoute = proxy.NewProxyRoute()

func TestCreateProxyContext(t *testing.T) {
	ctx := context.Background()
	req := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	proxy := proxyRoute.CreateProxyContext(ctx, req)

	assert.Equal(t, proxy.Ctx, ctx)
	assert.Same(t, proxy.Req, req)
	assert.Equal(t, proxy.RequestId, int64(1))
}

func TestProxyPassMethodGet(t *testing.T) {
	var server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(req.URL.Path))
	}))
	var link = server.URL + "/proxy-test"

	req := httptest.NewRequest(http.MethodGet, link, nil)
	rec := httptest.NewRecorder()
	proxyRoute.Handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, string(result), "/proxy-test")
}

func TestProxyPassMethodPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(req.URL.Path))
		io.Copy(w, req.Body)
		defer req.Body.Close()
	}))
	link := server.URL + "/proxy-test/"

	req := httptest.NewRequest(http.MethodPost, link, bytes.NewBufferString("post_data"))
	rec := httptest.NewRecorder()
	proxyRoute.Handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, string(result), "/proxy-test/post_data")
}

func TestHeaderXRequestId(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("lancache"))
	}))
	link := server.URL + "/proxy-test"

	req := httptest.NewRequest(http.MethodGet, link, nil)

	var resp *http.Response
	for range 3 {
		rec := httptest.NewRecorder()
		proxyRoute.Handler(rec, req)
		resp = rec.Result()
		resp.Body.Close()
	}

	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, resp.Header.Get("X-Request-Id"), strconv.FormatInt(proxyRoute.GetLastRequestId(), 10))
}

func TestServerSendStatusNotFound(t *testing.T) {
	var server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	var link = server.URL + "/proxy-test"
	req := httptest.NewRequest(http.MethodGet, link, nil)

	var resp *http.Response
	for range 3 {
		rec := httptest.NewRecorder()
		proxyRoute.Handler(rec, req)
		resp = rec.Result()
		resp.Body.Close()
	}

	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func TestProxyPassTLS(t *testing.T) {
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("lancache-tls"))
	}))

	req := httptest.NewRequest(http.MethodGet, s.URL+"/proxy-test", nil)
	rec := httptest.NewRecorder()
	proxyRoute.Handler(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
	assert.Equal(t, string(result), "lancache-tls")
}

func TestReplaceResponse(t *testing.T) {
	proxyRoute := proxy.NewProxyRoute()
	ctx := context.Background()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("lancache"))
	}))

	req := httptest.NewRequest(http.MethodGet, s.URL+"/proxy-test", nil)
	p := proxyRoute.CreateProxyContext(ctx, req)
	p.Intercept = func(proxy *proxy.Proxy) {
		p.Resp.StatusCode = http.StatusCreated
		p.Resp.Body = io.NopCloser(bytes.NewBufferString("proxy"))
	}

	rec := httptest.NewRecorder()
	proxyRoute.Pass(rec, p)

	resp := rec.Result()
	defer resp.Body.Close()

	result, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusCreated)
	assert.Equal(t, string(result), "proxy")
}

func init() {
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	slog.SetDefault(log)
}
