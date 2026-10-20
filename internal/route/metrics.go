package route

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsRoute struct {
	handler http.Handler
}

func newMetricsRoute() *MetricsRoute {
	return &MetricsRoute{
		handler: promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{Registry: prometheus.DefaultRegisterer}),
	}
}

func (r *MetricsRoute) Handler(w http.ResponseWriter, req *http.Request) bool {
	if req.URL.Path != "/metrics" {
		return false
	}

	w.Header().Add("X-Lancache-Status", "Metrics")
	r.handler.ServeHTTP(w, req)
	return true
}
