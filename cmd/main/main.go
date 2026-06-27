package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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
	sc := storage.NewStorageCache()
	ss := storage.NewStorageService(sr, sc, config.Storage)

	pp := proxy.NewProxyRoute()
	lr := lancache.NewLancacheRoute(ss, ts, pp)

	r := route.NewRoute()
	r.AddRoute(lr)
	r.AddRoute(pp)

	server := webserver.NewServer(config)
	server.Run(context.Background(), r)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop
}
