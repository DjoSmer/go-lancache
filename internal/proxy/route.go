package proxy

import (
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"
)

type ProxyRoute struct {
	requestSeq atomic.Int64
	Transport  *http.Transport
}

func NewProxyRoute() *ProxyRoute {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &ProxyRoute{
		Transport: &http.Transport{
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
			DialContext: dialer.DialContext,
		},
	}
}

func (ps *ProxyRoute) Handler(w http.ResponseWriter, req *http.Request) bool {
	if isLocalhost(req) {
		return false
	}

	requestId := ps.requestSeq.Add(1)
	w.Header().Add("X-Request-Id", strconv.FormatInt(requestId, 10))

	if w.Header().Get("X-Lancache-Status") == "" {
		w.Header().Add("X-Lancache-Status", "Proxy")
	}

	proxy := &httputil.ReverseProxy{
		Transport: ps.Transport,
		Rewrite:   func(pr *httputil.ProxyRequest) {},
	}
	proxy.ServeHTTP(w, req)

	return true
}

func (ps *ProxyRoute) GetLastRequestId() int64 {
	return ps.requestSeq.Load()
}

func (ps *ProxyRoute) log(name string) *slog.Logger {
	op := "ProxyRoute." + name
	return slog.Default().With(slog.String("op", op))
}

func isLocalhost(req *http.Request) bool {
	host := req.Host
	re := regexp.MustCompile(`^127\.0\.0\.1|localhost`)
	return re.MatchString(host)
}
