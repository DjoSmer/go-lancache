package lancache

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

type LancacheRoute struct {
	storageService storageService
	targetService  targetService
	proxyRoute     proxyRoute
	detector       *AnomalyDetector
}

func NewLancacheRoute(ss storageService, ts targetService, pr proxyRoute, detector *AnomalyDetector) *LancacheRoute {
	return &LancacheRoute{
		storageService: ss,
		targetService:  ts,
		proxyRoute:     pr,
		detector:       detector,
	}
}

func (r *LancacheRoute) Handler(w http.ResponseWriter, req *http.Request) bool {
	log := r.log("Handler")

	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	if req.Method != http.MethodGet || isLocalhost(req) {
		log.Debug("Skip: not method Get or Localhost")
		return false
	}

	t, err := r.targetService.Get(ctx, req.Host, req.UserAgent(), req.URL.Path)
	if err != nil {
		log.Debug("Skip: not target")
		return false
	}

	log.Debug("Run", slog.String("url", req.URL.Path))

	pathname := req.URL.Path
	basePath := t.Code + pathname

	storage, err := r.storageService.Get(ctx, basePath)
	if err != nil {
		storage = r.storageService.Create(basePath, t.Id)
	}

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

	if storage.IsSuccess() {
		storageFile, err := r.storageService.GetFile(storage)
		if err != nil {
			log.Error("Storage GetFile", "BasePath", storage.BasePath, "error", err.Error())
		} else {
			defer storageFile.Close()

			w.Header().Add("X-Lancache-Status", "HIT")
			io.Copy(w, storageFile)

			r.storageService.IncreaseDownloadCount(storage)
			r.detector.Finish(remoteIp, storage.BasePath)
			return true
		}
	}

	w.Header().Add("X-Lancache-Status", "MISS")

	proxy := r.proxyRoute.CreateProxyContext(ctx, req)

	if !storage.IsAvailable() {
		r.proxyRoute.Pass(w, proxy)
		return true
	}

	storage, result := r.storageService.Lock(storage)
	if !result {
		r.proxyRoute.Pass(w, proxy)
		return true
	}

	lancache := NewLancache(storage, r.storageService)
	proxy.Hooks = lancache

	r.proxyRoute.Pass(w, proxy)
	return true
}

func (r *LancacheRoute) log(name string) *slog.Logger {
	op := "LancacheRoute." + name
	return slog.Default().With(slog.String("op", op))
}

func isLocalhost(req *http.Request) bool {
	host := req.Host
	re := regexp.MustCompile(`^127\.0\.0\.1|localhost`)
	return re.MatchString(host)
}
