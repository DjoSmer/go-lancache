package lancache

import (
	"net/http"

	"go-lancache/internal/storage"

	"github.com/prometheus/client_golang/prometheus"
)

var metrics *responseWriterMetrics = newResponseWriterMetrics()

type LancacheResponseWriter struct {
	http.ResponseWriter
	storageFile storage.StorageFile
	storageErr  error
	// metrics     *responseWriterMetrics
}

func NewLancacheResponseWriter(storageFile storage.StorageFile, writer http.ResponseWriter) *LancacheResponseWriter {
	return &LancacheResponseWriter{writer, storageFile, nil}
}

func (w *LancacheResponseWriter) GetStorageError() error {
	return w.storageErr
}

func (w *LancacheResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *LancacheResponseWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if err != nil {
		metrics.writeErrors.Inc()
		return n, err
	}
	metrics.bytesSent.Add(float64(n))

	if n > 0 && w.storageErr == nil {
		n2, storageErr := w.storageFile.Write(p[:n])
		if storageErr != nil {
			w.storageErr = storageErr
			metrics.storageErrors.Inc()
		}
		metrics.bytesStorage.Add(float64(n2))
	}

	return n, err
}

type responseWriterMetrics struct {
	bytesSent     prometheus.Counter
	bytesStorage  prometheus.Counter
	writeErrors   prometheus.Counter
	storageErrors prometheus.Counter
}

func newResponseWriterMetrics() *responseWriterMetrics {
	m := &responseWriterMetrics{
		bytesSent: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "response_bytes_sent_total",
			},
		),
		bytesStorage: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "response_bytes_storage_total",
			},
		),
		writeErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "response_write_errors_total",
			},
		),
		storageErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "lancache",
				Name:      "response_storage_errors_total",
				Help:      "Total number of range detections.",
			},
		),
	}

	prometheus.MustRegister(
		m.bytesSent,
		m.bytesStorage,
		m.writeErrors,
		m.storageErrors,
	)

	return m
}
