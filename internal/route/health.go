package route

import (
	"io"
	"net/http"
)

type RouteHealth struct{}

func newRouteHealth() *RouteHealth {
	return &RouteHealth{}
}

func (r *RouteHealth) Handler(w http.ResponseWriter, req *http.Request) bool {
	if req.URL.Path != "/_lc_/health" {
		return false
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Add("X-Lancache-Status", "Health")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
	return true
}
