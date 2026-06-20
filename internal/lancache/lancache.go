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
	reader := proxy.Resp.Body
	l.StorageFile = l.storageService.CreateFile(l.storage, true)
	l.tee = io.TeeReader(reader, l.StorageFile)
	proxy.Resp.Body = l
}

func (l *lancache) AfterResponse(proxy *proxy.Proxy, err error) {
	log := l.log("AfterResponse")

	if err != nil {
		l.StorageFile.Close()
		err := l.StorageFile.Remove()
		if err != nil {
			log.Error("Cannot remove storage file - " + err.Error())
		}
	}

	l.storageService.Release(l.storage, err)
}

func (l *lancache) Read(p []byte) (n int, err error) {
	n, err = l.tee.Read(p)
	return n, err
}

func (l *lancache) Close() error {
	return l.StorageFile.Close()
}

func (l *lancache) log(name string) *slog.Logger {
	op := "Lancache." + name
	return slog.Default().With(slog.String("op", op))
}
