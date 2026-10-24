package lancache

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"go-lancache/internal/storage"

	"github.com/prometheus/client_golang/prometheus"
)

type downloadTask struct {
	Req     *http.Request
	Storage *storage.Storage
}

type LancacheDownload struct {
	ctx   context.Context
	tasks chan downloadTask
	// numberOfWorkers int
	storageService storageService
	metrics        *downloadMetrics
}

func NewLancacheDownload(ctx context.Context, numberOfWorkers int, storageService storageService) *LancacheDownload {
	download := &LancacheDownload{
		ctx:   ctx,
		tasks: make(chan downloadTask, numberOfWorkers*10),
		// numberOfWorkers: numberOfWorkers,
		storageService: storageService,
	}

	download.metrics = newDownloadMetrics(download)

	for i := range numberOfWorkers {
		go download.worker(i)
	}

	return download
}

func (d *LancacheDownload) Add(req *http.Request, storage *storage.Storage) {
	d.tasks <- downloadTask{
		Req:     req,
		Storage: storage,
	}
}

func (d *LancacheDownload) worker(workerId int) {
	for task := range d.tasks {
		if d.ctx.Err() != nil {
			d.storageService.Release(d.ctx, task.Storage, fmt.Errorf("Shutdown"))
			continue
		}

		n, err := d.downloadFile(d.ctx, task.Storage, task.Req, workerId)
		if n == 0 && err == nil {
			err = fmt.Errorf("File's empty")
		}

		if err == nil && d.ctx.Err() != nil {
			err = fmt.Errorf("Shutdown")
		}

		d.storageService.Release(d.ctx, task.Storage, err)
	}
}

func (d *LancacheDownload) downloadFile(ctx context.Context, storage *storage.Storage, req *http.Request, workerId int) (int64, error) {
	log := d.log("downloadFile").With("url", req.URL.String()).With("workerId", workerId)

	d.metrics.downloadsInProgress.Inc()
	defer d.metrics.downloadsInProgress.Dec()

	var n int64
	var res *http.Response

	file, err := d.storageService.GetFile(storage)
	if err != nil {
		log.Error("Storage GetFile In", "error", err.Error())
		return 0, err
	}
	defer file.Close()

	outReq := req.Clone(ctx)
	outReq.RequestURI = ""
	reqRange := outReq.Header.Get("Range")
	outReq.Header.Del("Range")

	log.Debug("Send request", "range", reqRange)
	d.metrics.started.Inc()

	started := time.Now()

	res, err = http.DefaultClient.Do(outReq)
	if err != nil {
		d.metrics.results.WithLabelValues("error").Inc()
		log.Error("Request error", "error", err)
		return n, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		d.metrics.results.WithLabelValues("bad_status").Inc()
		return 0, fmt.Errorf("upstream returned status %s", res.Status)
	}

	n, err = io.Copy(file, res.Body)
	if err != nil {
		log.Error("Copy error", "error", err)
		return n, err
	}

	d.metrics.downloadDuration.Observe(time.Since(started).Seconds())
	d.metrics.results.WithLabelValues("success").Inc()
	d.metrics.bytesDownloaded.Add(float64(n))

	log.Info(fmt.Sprintf("Downloaded %v bytes", n))

	return n, nil
}

func (d *LancacheDownload) TaskCount() int {
	return len(d.tasks)
}

func (d *LancacheDownload) log(name string) *slog.Logger {
	op := "LancacheDownload." + name
	return slog.Default().With(slog.String("op", op))
}

type downloadMetrics struct {
	queueSize           prometheus.GaugeFunc
	downloadsInProgress prometheus.Gauge
	started             prometheus.Counter
	results             *prometheus.CounterVec
	bytesDownloaded     prometheus.Counter
	downloadDuration    prometheus.Histogram
}

func newDownloadMetrics(d *LancacheDownload) *downloadMetrics {
	m := &downloadMetrics{
		queueSize: prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "download_queue_size",
				Help:      "Number of pending download jobs.",
			},
			func() float64 {
				return float64(d.TaskCount())
			},
		),
		downloadsInProgress: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "downloads_in_progress",
				Help:      "Number of active downloading.",
			},
		),
		started: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "downloads_started_total",
				Help:      "Total number of upstream download attempts started.",
			},
		),
		results: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "downloads_finished_total",
				Help:      "Total number of upstream download attempts by final result.",
			},
			[]string{"result"},
		),
		bytesDownloaded: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "download_bytes_total",
				Help:      "Total download volume in bytes.",
			},
		),
		downloadDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "download_duration_seconds",
				Help:      "Duration of upstream file downloads.",
				Buckets:   prometheus.ExponentialBuckets(0.5, 2, 14),
			},
		),
	}

	prometheus.MustRegister(
		m.queueSize,
		m.downloadsInProgress,
		m.started,
		m.results,
		m.bytesDownloaded,
		m.downloadDuration,
	)
	return m
}
