package lancache

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go-lancache/internal/storage"
)

type LancacheRoute struct {
	ctx            context.Context
	storageService storageService
	targetService  targetService
	proxyRoute     proxyRoute
	detector       *AnomalyDetector
}

func NewLancacheRoute(
	ctx context.Context,
	ss storageService,
	ts targetService,
	pr proxyRoute,
	detector *AnomalyDetector,
) *LancacheRoute {
	return &LancacheRoute{
		ctx:            ctx,
		storageService: ss,
		targetService:  ts,
		proxyRoute:     pr,
		detector:       detector,
	}
}

func (r *LancacheRoute) Handler(w http.ResponseWriter, req *http.Request) bool {
	log := r.log("Handler")
	ctx := req.Context()

	if req.Method != http.MethodGet {
		log.Debug("Skip: not method Get")
		return false
	}

	target, err := r.targetService.Get(ctx, req.Host, req.UserAgent(), req.URL.Path)
	if err != nil {
		log.Debug("Skip: not target")
		return false
	}

	log.Debug("Run", slog.String("url", req.URL.Path))

	pathname := req.URL.Path
	basePath := target.Code + pathname

	storage, err := r.storageService.Get(ctx, basePath)
	if err != nil {
		storage = r.storageService.Create(basePath, target.Id)
	}

	log = log.With(slog.String("BasePath", storage.BasePath))

	serveOk := r.tryServeFileFromStorage(w, req, storage)
	if serveOk {
		return true
	}

	w.Header().Add("X-Lancache-Status", "MISS")
	w.Header().Add("X-Storage-Status", string(storage.GetStatus()))

	if !storage.IsAvailable() {
		return false
	}

	storage, result := r.storageService.Lock(storage)
	if !result {
		return false
	}

	file, err := r.storageService.GetFile(storage)
	if err != nil {
		log.Error("Storage GetFile In", "error", err.Error())
		return false
	}
	if req.Header.Get("Range") != "" {
		go func() {
			defer file.Close()

			n, err := downloadFile(r.ctx, file, req)
			r.storageService.Release(ctx, storage, err)
			log.Debug(fmt.Sprintf("Downloaded %v bytes", n))
		}()
		return false
	}
	defer file.Close()

	lw := NewLancacheResponseWriter(file, w)
	defer func() {
		r.storageService.Release(ctx, storage, err)
	}()

	if !r.proxyRoute.Handler(lw, req) {
		err = fmt.Errorf("Proxy route return false")
		return false
	}

	err = lw.GetStorageError()
	if err != nil {
		log.Error("File write error", "error", err.Error())

		file.Close()
		err := file.Remove()
		if err != nil {
			log.Error("Cannot remove storage file", "error", err.Error())
		}
	}

	return true
}

func (r *LancacheRoute) tryServeFileFromStorage(w http.ResponseWriter, req *http.Request, storage *storage.Storage) bool {
	if !storage.IsSuccess() {
		return false
	}

	log := r.log("tryServeFileFromStorage").With(slog.String("BasePath", storage.BasePath))

	parts := strings.Split(req.RemoteAddr, ":")
	remoteIp := "unknown"
	if len(parts) > 0 {
		remoteIp = parts[0]
	}

	if storage.IsSuccess() {
		isAnomaly := r.detector.Detect(remoteIp, storage.BasePath)
		if isAnomaly {
			storage = r.storageService.MarkAsCorrupted(storage)
		}
	}

	if !storage.IsSuccess() {
		return false
	}

	file, err := r.storageService.GetFile(storage)
	if err != nil {
		log.Error("Storage GetFile Out", "error", err.Error())
		return false
	}
	defer file.Close()

	name := storage.BasePath
	modTime := time.Now()
	stat, err := file.Stat()
	if err != nil {
		log.Error("File Stat", "error", err.Error())
	} else if stat != nil {
		name = stat.Name()
		modTime = stat.ModTime()
	}

	w.Header().Add("X-Lancache-Status", "HIT")
	http.ServeContent(w, req, name, modTime, file)

	r.storageService.IncreaseDownloadCount(storage)
	r.detector.Finish(remoteIp, storage.BasePath)

	return true
}

func (r *LancacheRoute) log(name string) *slog.Logger {
	op := "LancacheRoute." + name
	return slog.Default().With(slog.String("op", op))
}

func downloadFile(ctx context.Context, file storage.StorageFile, req *http.Request) (int64, error) {
	log := slog.Default().With(slog.String("op", "downloadFile")).With("url", req.URL.String())

	outReq := req.Clone(ctx)
	outReq.Header.Del("Range")

	var n int64
	var err error
	var res *http.Response

	log.Debug("Send request")

	res, err = http.DefaultClient.Do(outReq)
	if err != nil {
		return n, err
	}

	log.Debug("Start download")

	n, err = io.Copy(file, res.Body)
	if err != nil {
		return n, err
	}

	return n, nil
}
