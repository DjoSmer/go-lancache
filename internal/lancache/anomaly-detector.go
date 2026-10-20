// Detect any anomalies when client downloads file from storage.
// Anomaly 1: If a client tries to download file many times > AnomalyDetectorConfig.HistoryMax it's anomaly and Detect return true
package lancache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go-lancache/internal/config"

	"github.com/prometheus/client_golang/prometheus"
)

type HistoryKey struct {
	Ip   string
	Path string
}

type history struct {
	Live         bool
	Times        int
	LastFinished time.Time
	RangeTimes   map[[32]byte]int
}

type AnomalyDetector struct {
	config            config.AnomalyDetectorConfig
	historyTTL        time.Duration
	cleanupTimeTicker time.Duration
	mu                sync.Mutex
	data              map[HistoryKey]*history
	stopWorker        chan struct{}
	metrics           *anomalyDetectorMetrics
}

func NewAnomalyDetector(config config.AnomalyDetectorConfig, historyTTL time.Duration, cleanupTimeTicker time.Duration) *AnomalyDetector {
	detector := &AnomalyDetector{
		config:            config,
		historyTTL:        historyTTL,
		cleanupTimeTicker: cleanupTimeTicker,
		data:              make(map[HistoryKey]*history, 10000),
		stopWorker:        make(chan struct{}),
	}

	detector.metrics = newAnomalyDetectorMetrics(detector)

	return detector
}

func (d *AnomalyDetector) Start(ctx context.Context) {
	go d.cleanupLoop(ctx)
}

func (d *AnomalyDetector) Stop() {
	close(d.stopWorker)
}

func (d *AnomalyDetector) Detect(ip, path, rangeHeader string) bool {
	log := d.log("Detect")

	d.metrics.checks.Inc()

	key := HistoryKey{
		Ip:   ip,
		Path: path,
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	h, ok := d.data[key]
	if !ok {
		h = &history{
			Times:        0,
			Live:         true,
			LastFinished: time.Now(),
			RangeTimes:   make(map[[32]byte]int, 100),
		}
	}

	times := 0
	h.Live = true

	if rangeHeader == "" {
		h.Times++
		times = h.Times
	} else {
		rangeKey := hashRange(rangeHeader)
		h.RangeTimes[rangeKey]++
		times = h.RangeTimes[rangeKey]
	}

	if times > d.config.HistoryMax {
		log.Info("Find Anomaly", "ip", ip, "key", key, "range", rangeHeader)
		delete(d.data, key)
		if rangeHeader == "" {
			d.metrics.detections.Inc()
		} else {
			d.metrics.rangeDetections.Inc()
		}
		return true
	}

	d.data[key] = h

	return false
}

func (d *AnomalyDetector) Finish(ip, path string) {
	key := HistoryKey{
		Ip:   ip,
		Path: path,
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	h, ok := d.data[key]
	if !ok {
		return
	}

	h.Live = false
	h.LastFinished = time.Now()
}

func (d *AnomalyDetector) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.data)
}

func (d *AnomalyDetector) Get(ip, path string) (history, error) {
	key := HistoryKey{
		Ip:   ip,
		Path: path,
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	h, ok := d.data[key]
	if !ok {
		return history{}, fmt.Errorf("Key: %s not found", key)
	}

	return *h, nil
}

func (d *AnomalyDetector) ActiveHistories() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.data)
}

func (d *AnomalyDetector) cleanupLoop(ctx context.Context) {
	log := d.log("cleanupLoop")
	ticker := time.NewTicker(d.cleanupTimeTicker)

	for {
		select {
		case <-ticker.C:
			d.mu.Lock()

			expireBefore := time.Now().Add(-d.historyTTL)

			for key, history := range d.data {
				if !history.Live && history.LastFinished.Before(expireBefore) {
					log.Debug("Cleanup", "key", key)
					delete(d.data, key)
				}
			}

			d.mu.Unlock()

		case <-d.stopWorker:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (d *AnomalyDetector) log(name string) *slog.Logger {
	op := "AnomalyDetector." + name
	return slog.Default().With(slog.String("op", op))
}

func hashRange(value string) [32]byte {
	h := sha256.New()

	h.Write([]byte(value))

	var result [32]byte
	copy(result[:], h.Sum(nil))

	return result
}

type anomalyDetectorMetrics struct {
	activeHistories prometheus.CounterFunc
	checks          prometheus.Counter
	detections      prometheus.Counter
	rangeDetections prometheus.Counter
}

func newAnomalyDetectorMetrics(detector *AnomalyDetector) *anomalyDetectorMetrics {
	m := &anomalyDetectorMetrics{
		activeHistories: prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "anomaly_active_histories",
				Help:      "Number of active anomaly histories.",
			},
			func() float64 {
				return float64(detector.ActiveHistories())
			},
		),
		checks: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "anomaly_checks_total",
				Help:      "Total number of checks.",
			},
		),
		detections: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "anomaly_detections_total",
				Help:      "Total number of detections.",
			},
		),
		rangeDetections: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "anomaly_range_detections_total",
				Help:      "Total number of range detections.",
			},
		),
	}

	prometheus.MustRegister(
		m.checks,
		m.detections,
		m.rangeDetections,
	)
	return m
}
