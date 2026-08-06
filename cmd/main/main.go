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
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	}))

	slog.SetDefault(log)

	config := config.Load()

	db := database.New(config.DBConfig)

	tr := target.NewDBTargetRepository(db)
	ts := target.NewTargetService(tr, config.Target)
	ts.Load()

	sr := storage.NewStorageRepository(db)
	sc := storage.NewStorageCache(time.Duration(config.Storage.CacheTTL) * time.Minute)
	ss := storage.NewStorageService(sr, sc, config.Storage)

	sc.OnEvict = func(s storage.Storage) {
		sr.Save(s)
	}
	sc.Start()

	pp := proxy.NewProxyRoute()

	ad := lancache.NewAnomalyDetector(config.AnomalyDetector, time.Duration(config.AnomalyDetector.HistoryTTL)*time.Second, time.Duration(config.AnomalyDetector.CleanupTimeTicker)*time.Second)
	ad.Start()
	lr := lancache.NewLancacheRoute(ss, ts, pp, ad)

	r := route.NewRoute()
	r.AddRoute(lr)
	r.AddRoute(pp)

	server := webserver.NewServer(config)
	server.Run(context.Background(), r)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	server.Close()
	sc.Stop()
	ad.Stop()

	sc.EvictAll()
}
