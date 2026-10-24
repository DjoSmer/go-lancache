package route

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type RouteHandle interface {
	Handler(w http.ResponseWriter, req *http.Request) bool
}

type Route struct {
	mu      sync.RWMutex
	routes  []RouteHandle
	metrics *routeMetrics
}

func NewRoute() *Route {
	routes := make([]RouteHandle, 0, 8)
	routes = append(routes, newRouteHealth(), newMetricsRoute())

	return &Route{
		routes:  routes,
		metrics: newRouteMetrics(),
	}
}

func (r *Route) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log := r.log("ServeHTTP")

	req.URL.Host = req.Host
	req.URL.Scheme = "http"
	if req.TLS != nil {
		req.URL.Scheme += "s"
	}

	r.metrics.requestsInProgress.Inc()
	defer r.metrics.requestsInProgress.Dec()

	rw := NewRouteResponseWriter(w)
	started := time.Now()
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

	if req.Header.Get("Range") != "" {
		r.metrics.rangeRequests.Inc()
	}
	r.metrics.responseDuration.WithLabelValues(strings.ToLower(lancacheStatus)).Observe(time.Since(started).Seconds())
	r.metrics.requests.WithLabelValues(strings.ToLower(lancacheStatus)).Inc()
	r.metrics.responseBytes.WithLabelValues(strings.ToLower(lancacheStatus)).Add(float64(rw.contentSize))
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
	statusCode  int
	contentSize int
}

func NewRouteResponseWriter(w http.ResponseWriter) *routeResponseWriter {
	return &routeResponseWriter{w, http.StatusOK, 0}
}

func (rw *routeResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *routeResponseWriter) Write(p []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(p)
	rw.contentSize += n
	return n, err
}

func (rw *routeResponseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type routeMetrics struct {
	requests           *prometheus.CounterVec   // metrics, hit, miss, proxy
	responseBytes      *prometheus.CounterVec   // metrics, hit, miss, proxy
	responseDuration   *prometheus.HistogramVec // metrics, hit, miss, proxy
	requestsInProgress prometheus.Gauge
	rangeRequests      prometheus.Counter
}

func newRouteMetrics() *routeMetrics {
	m := &routeMetrics{
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "route",
				Name:      "requests_total",
				Help:      "Total number of requests.",
			},
			[]string{"source"},
		),
		responseBytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "route",
				Name:      "response_bytes_total",
				Help:      "Total number of response bytes attempts by sources.",
			},
			[]string{"source"},
		),
		responseDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "lc",
				Subsystem: "route",
				Name:      "requests_duration_seconds",
				Buckets:   prometheus.ExponentialBuckets(0.5, 2, 14),
			},
			[]string{"source"},
		),
		requestsInProgress: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "lc",
				Subsystem: "route",
				Name:      "requests_in_progress",
			},
		),
		rangeRequests: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "route",
				Name:      "range_requests_total",
				Help:      "Total number of only range requests.",
			},
		),
	}

	prometheus.MustRegister(
		m.requests,
		m.responseBytes,
		m.responseDuration,
		m.rangeRequests,
		m.requestsInProgress,
	)

	return m
}
