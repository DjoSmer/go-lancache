package route

import (
	"log/slog"
	"net/http"
	"sync"
)

type RouteHandle interface {
	Handler(w http.ResponseWriter, req *http.Request) bool
}

type Route struct {
	mu     sync.RWMutex
	routes []RouteHandle
}

func NewRoute() *Route {
	routes := make([]RouteHandle, 0, 8)
	routes = append(routes, newRouteHealth(), newMetricsRoute())

	return &Route{
		routes: routes,
	}
}

func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log := r.log("ServeHTTP")

	req.URL.Host = req.Host
	req.URL.Scheme = "http"
	if req.TLS != nil {
		req.URL.Scheme += "s"
	}

	rw := NewRouteResponseWriter(w)

	found := false
	for _, route := range r.routes {
		res := route.Handler(rw, req)
		if res {
			found = true
			break
		}
	}

	if !found {
		r.routeNotFound(rw, req)
	}

	lancacheStatus := w.Header().Get("X-Lancache-Status")

	log.Info(req.Method, "url", req.URL.String(), "code", rw.statusCode, "userAgent", req.UserAgent(), "LStatus", lancacheStatus)
}

func (r *Route) AddRoute(h RouteHandle) {
	r.routes = append(r.routes, h)
}

func (r *Route) routeNotFound(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Status", "NotFound")
	http.NotFound(w, req)
}

func (r *Route) log(name string) *slog.Logger {
	op := "Route." + name
	return slog.Default().With(slog.String("op", op))
}

type routeResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewRouteResponseWriter(w http.ResponseWriter) *routeResponseWriter {
	return &routeResponseWriter{w, http.StatusOK}
}

func (rw *routeResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *routeResponseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
