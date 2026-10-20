package lancache

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go-lancache/internal/config"
	"go-lancache/internal/storage"

	"github.com/prometheus/client_golang/prometheus"
)

type LancacheRoute struct {
	ctx            context.Context
	storageService storageService
	targetService  targetService
	proxyRoute     proxyRoute
	detector       *AnomalyDetector
	download       *LancacheDownload
	metrics        *routeMetrics
}

func NewLancacheRoute(
	ctx context.Context,
	config config.Config,
	ss storageService,
	ts targetService,
	pr proxyRoute,
	detector *AnomalyDetector,
) *LancacheRoute {
	return &LancacheRoute{
		ctx:            ctx,
		storageService: ss,
		targetService:  ts,
		proxyRoute:     pr,
		detector:       detector,
		download:       NewLancacheDownload(ctx, config.DownloadConfig.NumberOfWorkers, ss),
		metrics:        newRouteMetrics(),
	}
}

func (r *LancacheRoute) Handler(w http.ResponseWriter, req *http.Request) bool {
	log := r.log("Handler")
	ctx := req.Context()

	if req.Method != http.MethodGet {
		log.Debug("Skip: not method Get")
		return false
	}

	target, err := r.targetService.Get(ctx, req.Host, req.UserAgent(), req.URL.Path)
	if err != nil {
		log.Debug("Skip: not target")
		return false
	}

	log.Debug("Run", slog.String("url", req.URL.Path))

	pathname := req.URL.Path
	basePath := target.Code + pathname

	storage, err := r.storageService.Get(ctx, basePath)
	if err != nil {
		storage = r.storageService.Create(basePath, target.Id)
	}

	log = log.With(slog.String("BasePath", storage.BasePath))

	if req.Header.Get("Range") != "" {
		r.metrics.rangeRequests.Inc()
	} else {
		r.metrics.requests.Inc()
	}

	if r.tryServeFileFromStorage(w, req, storage) {
		return true
	}

	w.Header().Add("X-Lancache-Status", "MISS")
	w.Header().Add("X-Storage-Status", string(storage.GetStatus()))

	if !storage.IsAvailable() {
		return false
	}

	storage, result := r.storageService.Lock(storage)
	if !result {
		return false
	}

	if req.Header.Get("Range") != "" {
		r.download.Add(req, storage)
		return false
	}

	file, err := r.storageService.GetFile(storage)
	if err != nil {
		log.Error("Storage GetFile In", "error", err.Error())
		return false
	}

	lw := NewLancacheResponseWriter(file, w)
	defer func() {
		isIncMetric := false
		closeErr := file.Close()

		err = lw.GetStorageError()

		if err == nil && ctx.Err() != nil {
			isIncMetric = true
			r.metrics.miss.WithLabelValues("disconnect").Inc()
			err = ctx.Err()
		}

		if closeErr != nil && err == nil {
			err = closeErr
		}

		if err != nil {
			log.Error("Storage file was not saved", "error", err)

			if !isIncMetric {
				r.metrics.miss.WithLabelValues("error").Inc()
			}

			fileErr := file.Remove()
			if fileErr != nil {
				log.Error("Cannot remove storage file", "error", fileErr)
			}
		}

		r.storageService.Release(r.ctx, storage, err)
	}()

	if !r.proxyRoute.Handler(lw, req) {
		err = fmt.Errorf("Proxy route return false")
		log.Error(err.Error())
		return false
	}

	contentLength, err := strconv.Atoi(w.Header().Get("Content-Length"))
	if err == nil && contentLength > 0 {
		r.metrics.responseBytes.WithLabelValues("upstream").Add(float64(contentLength))
	}
	r.metrics.miss.WithLabelValues("success").Inc()

	return true
}

func (r *LancacheRoute) tryServeFileFromStorage(w http.ResponseWriter, req *http.Request, storage *storage.Storage) bool {
	if !storage.IsSuccess() {
		return false
	}

	log := r.log("tryServeFileFromStorage").With(slog.String("BasePath", storage.BasePath))

	parts := strings.Split(req.RemoteAddr, ":")
	remoteIp := "unknown"
	if len(parts) > 0 {
		remoteIp = parts[0]
	}

	isAnomaly := r.detector.Detect(remoteIp, storage.BasePath, req.Header.Get("Range"))
	if isAnomaly {
		storage = r.storageService.MarkAsCorrupted(storage)
	}

	if !storage.IsSuccess() {
		return false
	}

	file, err := r.storageService.GetFile(storage)
	if err != nil {
		log.Error("Storage GetFile Out", "error", err.Error())
		r.metrics.hit.WithLabelValues("error").Inc()
		return false
	}
	defer file.Close()
	defer func() {
		if req.Context().Err() != nil {
			r.metrics.hit.WithLabelValues("disconnect").Inc()
		} else {
			r.metrics.hit.WithLabelValues("success").Inc()
		}
	}()

	name := storage.BasePath
	modTime := time.Now()
	stat, err := file.Stat()
	if err != nil {
		log.Error("File Stat", "error", err.Error())
	} else if stat != nil {
		name = stat.Name()
		modTime = stat.ModTime()
	}

	w.Header().Add("X-Lancache-Status", "HIT")
	http.ServeContent(w, req, name, modTime, file)

	r.storageService.IncreaseDownloadCount(storage)
	r.detector.Finish(remoteIp, storage.BasePath)

	contentLength, err := strconv.Atoi(w.Header().Get("Content-Length"))
	if err == nil && contentLength > 0 {
		r.metrics.responseBytes.WithLabelValues("storage").Add(float64(contentLength))
	}

	return true
}

func (r *LancacheRoute) log(name string) *slog.Logger {
	op := "LancacheRoute." + name
	return slog.Default().With(slog.String("op", op))
}

type routeMetrics struct {
	requests      prometheus.Counter
	rangeRequests prometheus.Counter
	hit           *prometheus.CounterVec //success, error, disconnect
	miss          *prometheus.CounterVec //success, error, disconnect

	responseBytes *prometheus.CounterVec //storage, upstream
}

func newRouteMetrics() *routeMetrics {
	m := &routeMetrics{
		requests: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "route_requests_total",
				Help:      "Total number of lancache requests.",
			},
		),
		rangeRequests: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "route_range_requests_total",
				Help:      "Total number of lancache range requests.",
			},
		),
		hit: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "route_hit_finished_total",
				Help:      "Total number of hit download by final result.",
			},
			[]string{"result"},
		),
		miss: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "route_miss_finished_total",
				Help:      "Total number of miss download by final result.",
			},
			[]string{"result"},
		),
		responseBytes: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "route_response_bytes_total",
				Help:      "Total number of download bytes attempts by storage and upstream.",
			},
			[]string{"source"},
		),
	}

	prometheus.MustRegister(
		m.requests,
		m.rangeRequests,
		m.hit,
		m.miss,
		m.responseBytes,
	)

	return m
}
