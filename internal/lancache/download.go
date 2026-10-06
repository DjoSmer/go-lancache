package lancache

import (
	"context"
	"fmt"
	"go-lancache/internal/storage"
	"io"
	"log/slog"
	"net/http"
)

type downloadTask struct {
	Req     *http.Request
	Storage *storage.Storage
}

type LancacheDownload struct {
	ctx             context.Context
	tasks           chan downloadTask
	numberOfWorkers int
	storageService  storageService
}

func NewLancacheDownload(ctx context.Context, numberOfWorkers int, storageService storageService) *LancacheDownload {
	download := &LancacheDownload{
		ctx:             ctx,
		tasks:           make(chan downloadTask, numberOfWorkers*10),
		numberOfWorkers: numberOfWorkers,
		storageService:  storageService,
	}

	for range numberOfWorkers {
		go download.worker()
	}

	return download
}

func (d *LancacheDownload) Add(req *http.Request, storage *storage.Storage) {
	d.tasks <- downloadTask{
		Req:     req,
		Storage: storage,
	}
}

func (d *LancacheDownload) worker() {
	for task := range d.tasks {
		if d.ctx.Err() != nil {
			d.storageService.Release(d.ctx, task.Storage, fmt.Errorf("Shutdown"))
			continue
		}

		n, err := d.downloadFile(d.ctx, task.Storage, task.Req)
		if n == 0 && err == nil {
			err = fmt.Errorf("File's empty")
		}

		if err == nil && d.ctx.Err() != nil {
			err = fmt.Errorf("Shutdown")
		}

		d.storageService.Release(d.ctx, task.Storage, err)
	}
}

func (d *LancacheDownload) downloadFile(ctx context.Context, storage *storage.Storage, req *http.Request) (int64, error) {
	log := d.log("downloadFile").With("url", req.URL.String())

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

	res, err = http.DefaultClient.Do(outReq)
	if err != nil {
		log.Debug("Request error", "error", err)
		return n, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return 0, fmt.Errorf("upstream returned status %s", res.Status)
	}

	n, err = io.Copy(file, res.Body)
	if err != nil {
		log.Debug("Copy error", "error", err)
		return n, err
	}

	log.Debug(fmt.Sprintf("Downloaded %v bytes", n))

	return n, nil
}

func (d *LancacheDownload) log(name string) *slog.Logger {
	op := "LancacheDownload." + name
	return slog.Default().With(slog.String("op", op))
}
