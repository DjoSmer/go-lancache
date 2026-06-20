package proxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type ProxyRoute struct {
	requestSeq int64
	transport  *http.Transport
}

func NewProxyRoute() *ProxyRoute {
	return &ProxyRoute{
		requestSeq: 0,
		transport: &http.Transport{
			MaxIdleConns:          1000,
			MaxIdleConnsPerHost:   100,
			MaxConnsPerHost:       0,
			ForceAttemptHTTP2:     true,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

func (ps *ProxyRoute) CreateProxyContext(ctx context.Context, req *http.Request) *Proxy {
	return &Proxy{
		Ctx:       ctx,
		Req:       req,
		RequestId: atomic.AddInt64(&ps.requestSeq, 1),
	}
}

func (ps *ProxyRoute) Handler(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	proxy := ps.CreateProxyContext(ctx, req)
	ps.Pass(w, proxy)
}

func (ps *ProxyRoute) Pass(w http.ResponseWriter, proxy *Proxy) {
	log := ps.log("Pass").With(
		"host", proxy.Req.Host,
		"uri", proxy.Req.URL.RequestURI(),
	)

	w.Header().Add("X-Request-Id", strconv.FormatInt(proxy.RequestId, 10))

	if proxy.Hooks != nil {
		proxy.Hooks.BeforeRequest(proxy)
	}

	resp, err := ps.transport.RoundTrip(proxy.Req)
	if err != nil {
		log.Debug("Error response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if resp == nil {
		message := "Response is null"
		log.Debug(message)
		http.Error(w, message, http.StatusInternalServerError)
		return
	}

	proxy.Resp = resp
	var origBody io.ReadCloser = resp.Body
	defer origBody.Close()

	if proxy.Hooks != nil {
		proxy.Hooks.InterceptResponse(proxy)
	}

	dst := w.Header()
	for k, vs := range resp.Header {
		dst[k] = append(dst[k], vs...)
	}

	w.WriteHeader(resp.StatusCode)

	nr, err := io.Copy(w, resp.Body)

	if proxy.Hooks != nil {
		proxy.Hooks.AfterResponse(proxy, err)
	}

	if err != nil {
		log.Warn("Cannot copy to copy writer", "error", err)
		return
	}

	err = resp.Body.Close()
	if err != nil {
		log.Warn("Cannot close response body", "error", err)
	}

	log.Debug(fmt.Sprintf("Copied %v bytes to client", nr))
}

func (ps *ProxyRoute) GetLastRequestId() int64 {
	return ps.requestSeq
}

func (ps *ProxyRoute) log(name string) *slog.Logger {
	op := "ProxyRoute." + name
	return slog.Default().With(slog.String("op", op))
}
