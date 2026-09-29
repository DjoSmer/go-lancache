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
)

func main() {
	config := config.Load()

	var level slog.Level
	err := level.UnmarshalText([]byte(config.Log.Level))
	if err != nil {
		level = slog.LevelInfo
	}

	ctx := context.Background()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: config.Log.AddSource,
		Level:     level,
	}))

	log.Info("Lancache Running...")
	slog.SetDefault(log)

	db := database.New(config.DBConfig)

	targetRepository := target.NewTargetRepository(db)
	targetService := target.NewTargetService(targetRepository, config.Target)
	targetService.Load()

	storageRepository := storage.NewStorageRepository(db)
	storageCleaner := storage.NewStorageCleaner(storageRepository, config.Storage)
	_, err = storageCleaner.CalculateStorageSize()
	if err != nil {
		panic(err)
	}
	storageCleaner.Start(ctx)

	storageCache := storage.NewStorageCache(time.Duration(config.Storage.CacheTTL) * time.Minute)
	storageService := storage.NewStorageService(storageRepository, storageCache, config.Storage)

	storageCache.OnEvict = func(s storage.Storage) {
		storageRepository.Save(ctx, s)
	}
	storageCache.Start(ctx)

	proxyRoute := proxy.NewProxyRoute()

	anomalyDetector := lancache.NewAnomalyDetector(config.AnomalyDetector, time.Duration(config.AnomalyDetector.HistoryTTL)*time.Second, time.Duration(config.AnomalyDetector.CleanupTimeTicker)*time.Second)
	anomalyDetector.Start(ctx)
	lancacheRoute := lancache.NewLancacheRoute(ctx, storageService, targetService, proxyRoute, anomalyDetector)

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

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	server.Shutdown(ctx)
	storageCleaner.Stop()
	anomalyDetector.Stop()
	storageCache.EvictAll(ctx)

}
