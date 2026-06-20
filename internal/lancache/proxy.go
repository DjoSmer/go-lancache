package lancache

import (
	"context"
	"net/http"

	"go-lancache/internal/proxy"
)

type proxyRoute interface {
	CreateProxyContext(ctx context.Context, req *http.Request) *proxy.Proxy
	Pass(w http.ResponseWriter, proxy *proxy.Proxy)
}
