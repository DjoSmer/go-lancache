package route

import (
	"net/http"
)

type RouteHandle interface {
	Handler(w http.ResponseWriter, req *http.Request) bool
}

type Route struct {
	routes []RouteHandle
}

func NewRoute() *Route {
	return &Route{
		routes: make([]RouteHandle, 0, 8),
	}
}

func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	req.URL.Host = req.Host
	req.URL.Scheme = "http"
	if req.TLS != nil {
		req.URL.Scheme += "s"
	}

	for _, route := range r.routes {
		res := route.Handler(w, req)
		if res {
			break
		}
	}
}

func (r *Route) AddRoute(h RouteHandle) {
	r.routes = append(r.routes, h)
}
