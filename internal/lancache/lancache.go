package lancache

import (
	"io"
	"log/slog"

	"go-lancache/internal/proxy"
	"go-lancache/internal/storage"
)

type lancache struct {
	StorageFile    storage.StorageFile
	storage        *storage.Storage
	storageService storageService
	tee            io.Reader
}

func NewLancache(storage *storage.Storage, ss storageService) *lancache {
	return &lancache{
		storage:        storage,
		storageService: ss,
	}
}

func (l *lancache) BeforeRequest(proxy *proxy.Proxy) {}

func (l *lancache) InterceptResponse(proxy *proxy.Proxy) {
	log := l.log("InterceptResponse")

	if proxy == nil {
		log.Warn("Proxy is nil")
		return
	}

	log.Debug("Run", slog.String("url", proxy.Req.URL.Path))

	reader := proxy.Resp.Body
	storageFile, err := l.storageService.GetFile(l.storage)
	if err != nil {
		log.Error("Storage GetFile", "BasePath", l.storage.BasePath, "error", err.Error())
		return
	}
	l.StorageFile = storageFile
	l.tee = io.TeeReader(reader, l.StorageFile)
	proxy.Resp.Body = l
}

func (l *lancache) AfterResponse(proxy *proxy.Proxy, err error) {
	log := l.log("AfterResponse")

	if proxy == nil {
		log.Warn("Proxy is nil")
		return
	}

	log.Debug("Run", slog.String("url", proxy.Req.URL.Path))
	//TODO add Content-Length check
	if err != nil {
		log.Debug("Error: " + err.Error())

		l.StorageFile.Close()
		err := l.StorageFile.Remove()
		if err != nil {
			log.Error("Cannot remove storage file - " + err.Error())
		}
	}

	l.storageService.Release(proxy.Ctx, l.storage, err)
}

func (l *lancache) Read(p []byte) (n int, err error) {
	n, err = l.tee.Read(p)
	return n, err
}

func (l *lancache) Close() error {
	if l.StorageFile == nil {
		return nil
	}
	return l.StorageFile.Close()
}

func (l *lancache) log(name string) *slog.Logger {
	op := "Lancache." + name
	return slog.Default().With(slog.String("op", op))
}
