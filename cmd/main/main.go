package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-lancache/internal/config"
	"go-lancache/internal/database"
	"go-lancache/internal/lancache"
	"go-lancache/internal/proxy"
	"go-lancache/internal/route"
	"go-lancache/internal/storage"
	"go-lancache/internal/target"
	"go-lancache/internal/webserver"
	"go-lancache/pkg/flog"
)

func main() {
	config := config.Load()

	var level slog.Level
	err := level.UnmarshalText([]byte(config.Log.Level))
	if err != nil {
		level = slog.LevelInfo
	}

	ctx, cancel := context.WithCancel(context.Background())

	baseHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: config.Log.AddSource,
		Level:     level,
	})

	filterHandler := flog.NewFilterHandler(baseHandler, config.Log.DebugPrefix)
	log := slog.New(filterHandler)

	log.Info("Lancache Running...")
	slog.SetDefault(log)

	db := database.New(config.DBConfig)

	targetRepository := target.NewTargetRepository(db)
	storageRepository := storage.NewStorageRepository(db)

	targetService := target.NewTargetService(targetRepository, config.Target)
	err = targetService.Load()
	if err != nil {
		panic(err)
	}

	storageCleaner := storage.NewStorageCleaner(storageRepository, config.Storage)
	_, err = storageCleaner.CalculateStorageSize()
	if err != nil {
		panic(err)
	}
	storageCleaner.Start(ctx)

	storageCache := storage.NewStorageCache(time.Duration(config.Storage.CacheTTL) * time.Minute)
	storageCache.OnEvict = func(s storage.Storage) {
		// if ctx is cancel lancache can't save storage in cache
		storageRepository.Save(context.TODO(), s)
	}
	storageCache.Start(ctx)

	storageService := storage.NewStorageService(storageRepository, storageCache, config.Storage)
	proxyRoute := proxy.NewProxyRoute()

	anomalyDetector := lancache.NewAnomalyDetector(config.AnomalyDetector, time.Duration(config.AnomalyDetector.HistoryTTL)*time.Second, time.Duration(config.AnomalyDetector.CleanupTimeTicker)*time.Second)
	anomalyDetector.Start(ctx)

	lancacheRoute := lancache.NewLancacheRoute(ctx, config, storageService, targetService, proxyRoute, anomalyDetector)

	route := route.NewRoute()
	route.AddRoute(lancacheRoute)
	route.AddRoute(proxyRoute)

	server := webserver.NewServer(config)
	errors := server.Run(ctx, route)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case signal := <-signals:
		log.Info("Lancache Shutdown", "signal", signal)
	case serveErr := <-errors:
		log.Error("Listener failed", "error", serveErr)
	}

	ctx, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	server.Shutdown(ctx)
	cancel()

	// LancacheDownload has pools and first need their storages release, after StorageCache save it
	// better use wg
	time.After(2 * time.Second)

	storageCache.EvictAll(ctx)
}
