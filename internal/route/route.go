package route

import (
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
	routes = append(routes, newRouteHealth())

	return &Route{
		routes: routes,
	}
}

func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.URL.Host = req.Host
	req.URL.Scheme = "http"
	if req.TLS != nil {
		req.URL.Scheme += "s"
	}

	found := false
	for _, route := range r.routes {
		res := route.Handler(w, req)
		if res {
			found = true
			break
		}
	}

	if !found {
		r.routeNotFound(w, req)
	}
}

func (r *Route) AddRoute(h RouteHandle) {
	r.routes = append(r.routes, h)
}

func (r *Route) routeNotFound(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Status", "NotFound")
	http.NotFound(w, req)
}
