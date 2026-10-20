package storage

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go-lancache/internal/config"

	"github.com/prometheus/client_golang/prometheus"
)

var regexpFileTTL = regexp.MustCompile(`^([0-9.]*)([mwd])$`)
var regexpDiskSize = regexp.MustCompile(`^([0-9.]*)([TGM])$`)

type StorageCleanupRepository interface {
	GetExpired(context.Context, time.Duration) ([]Storage, error)
	GetOldest(ctx context.Context, limit int) ([]Storage, error)
	DeleteMany(context.Context, []Storage) error
}

type StorageCleaner struct {
	repo       StorageCleanupRepository
	config     config.StorageConfig
	fileTTL    time.Duration
	maxSize    int64
	stopWorker chan struct{}
	metrics    *cleanerMetrics
}

func NewStorageCleaner(repo StorageCleanupRepository, config config.StorageConfig) *StorageCleaner {
	fileTTL, err := parseFileTTL(config.FileTTL)
	if err != nil {
		panic(err)
	}

	maxSize, err := parseDiskSize(config.DiskSize)
	if err != nil {
		panic(err)
	}

	fmt.Println(fileTTL, maxSize, config)
	return &StorageCleaner{
		repo:       repo,
		config:     config,
		fileTTL:    fileTTL,
		maxSize:    maxSize,
		stopWorker: make(chan struct{}),
		metrics:    newCleanerMetrics(),
	}
}

func (c *StorageCleaner) CalculateStorageSize() (int64, error) {
	log := c.log("CalculateStorageSize")

	var size int64 = 0
	err := filepath.WalkDir(c.config.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				log.Warn("Cannot get file info", "path", path)
				return nil
			}
			size += info.Size()
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	c.metrics.storageBytes.Set(float64(size))
	log.Info(fmt.Sprintf("Storage Size: %v / %v", sizeFormat(size), c.config.DiskSize))

	return size, nil
}

func (c *StorageCleaner) cleanup(ctx context.Context) {
	log := c.log("cleanup")

	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()
	//TODO: if ctx is done cleanup has to finish before app close
	for {
		select {
		case <-ticker.C:
			c.metrics.cleanupRuns.Inc()

			storages, err := c.repo.GetExpired(ctx, c.fileTTL)
			if err != nil {
				log.Error("repo.GetExpired", "error", err)
				c.metrics.cleanupErrors.Inc()
			} else {
				c.remove(ctx, storages)
			}

			err = c.cleanupOldest(ctx)
			if err != nil {
				log.Error(err.Error())
				c.metrics.cleanupErrors.Inc()
			}

		case <-c.stopWorker:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *StorageCleaner) cleanupOldest(ctx context.Context) error {
	log := c.log("cleanupOldest")
	currentSize, err := c.CalculateStorageSize()
	if err != nil {
		log.Error(err.Error())
		return err
	}

	if currentSize < c.maxSize {
		return nil
	}

	var deletedBytes int64
	maxSize := int64(float64(c.maxSize) * 0.9)
	for range 100 {
		storages, err := c.repo.GetOldest(ctx, 1000)
		if err != nil {
			break
		}

		deletedBytes += c.remove(ctx, storages)
		if currentSize-deletedBytes < maxSize || ctx.Err() != nil {
			break
		}
	}

	if deletedBytes > 0 {
		log.Info("Cleanup oldest result", "currentSize", sizeFormat(currentSize), "deletedSize", sizeFormat(deletedBytes), "maxSize", maxSize)
	}

	return err
}

func (c *StorageCleaner) remove(ctx context.Context, storages []Storage) int64 {
	log := c.log("remove")
	if len(storages) == 0 {
		return 0
	}

	var deletedFiles int
	var deletedBytes int64
	toDelete := make([]Storage, 0, 100)

	for _, storage := range storages {
		storageFile, err := NewStorageFile(&storage, c.config.Path)
		if err != nil {
			log.Error("StorageFile", "BasePath", storage.BasePath, "error", err.Error())
			continue
		}

		st, _ := storageFile.Stat()
		if st != nil {
			deletedBytes += st.Size()
		}

		err = storageFile.Remove()
		if err != nil {
			c.metrics.deletedErrors.Inc()
			log.Error("Cannot remove file", "BasePath", storage.BasePath, "error", err.Error())
			continue
		}

		toDelete = append(toDelete, storage)

		if len(toDelete) >= 100 {
			c.repo.DeleteMany(ctx, toDelete)
			toDelete = toDelete[:0]
			deletedFiles += 100
		}
	}

	c.metrics.deletedBytes.Add(float64(deletedBytes))
	c.metrics.deletedFiles.Add(float64(deletedFiles))
	log.Info(fmt.Sprintf("Deleted %d / %d files = %v", deletedFiles, len(storages), sizeFormat(deletedBytes)))

	return deletedBytes
}

func (c *StorageCleaner) Start(ctx context.Context) {
	go c.cleanup(ctx)
}

func (c *StorageCleaner) Stop() {
	close(c.stopWorker)
}

func (c *StorageCleaner) log(name string) *slog.Logger {
	op := "StorageCleaner." + name
	return slog.Default().With(slog.String("op", op))
}

func parseFileTTL(baseFileTTL string) (time.Duration, error) {
	match := regexpFileTTL.FindStringSubmatch(baseFileTTL)
	if len(match) == 0 {
		return 0, fmt.Errorf("Parse FileTTL doesn't match mask 1m/4w/31d")
	}

	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("Parse FileTTL: %w", err)
	}

	duration := 24 * time.Hour // 1 day
	unit := match[2]
	switch unit {
	case "w":
		duration *= 7 // 1 week
	case "m":
		duration *= 30 // 1 month
	}

	return duration * time.Duration(value), nil
}

func parseDiskSize(baseDiskSize string) (int64, error) {
	match := regexpDiskSize.FindStringSubmatch(baseDiskSize)
	if len(match) == 0 {
		return 0, fmt.Errorf("Parse Disk Size doesn't match mask 1T/1024G/10240M")
	}

	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("Parse Disk Size: %w", err)
	}

	scale := 1024 // KB
	unit := strings.ToUpper(match[2])
	switch unit {
	case "T":
		scale = scale * scale * scale * scale // TB
	case "G":
		scale = scale * scale * scale // GB
	case "M":
		scale = scale * scale // MB
	}

	return int64(value * scale), nil
}

func sizeFormat(size int64) string {
	units := []string{"B", "K", "M", "G", "T"}

	for _, unit := range units {
		if size <= 1024 {
			return fmt.Sprintf("%d%v", size, unit)
		}
		size /= 1024
	}

	return strconv.FormatInt(size, 10)
}

type cleanerMetrics struct {
	storageBytes prometheus.Gauge

	deletedFiles  prometheus.Counter
	deletedErrors prometheus.Counter
	deletedBytes  prometheus.Counter

	cleanupRuns   prometheus.Counter
	cleanupErrors prometheus.Counter
}

func newCleanerMetrics() *cleanerMetrics {
	m := &cleanerMetrics{
		storageBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "lc",
				Subsystem: "storage",
				Name:      "cleaner_storage_bytes_total",
			},
		),
		deletedFiles: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "storage",
				Name:      "cleaner_deleted_files_total",
			},
		),
		deletedErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "storage",
				Name:      "cleaner_deleted_errors_total",
			},
		),
		deletedBytes: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "storage",
				Name:      "cleaner_deleted_bytes_total",
			},
		),
		cleanupRuns: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "storage",
				Name:      "cleaner_cleanup_runs_total",
			},
		),
		cleanupErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: "lc",
				Subsystem: "storage",
				Name:      "cleaner_cleanup_errors_total",
			},
		),
	}

	prometheus.MustRegister(
		m.storageBytes,
		m.deletedFiles,
		m.deletedErrors,
		m.deletedBytes,
		m.cleanupRuns,
		m.cleanupErrors,
	)

	return m
}
