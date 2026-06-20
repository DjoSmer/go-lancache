package lancache

import (
	"context"
	"io"
	"net/http"
	"regexp"
)

type LancacheRoute struct {
	storageService storageService
	targetService  targetService
	proxyRoute     proxyRoute
}

func NewLancacheRoute(ss storageService, ts targetService, pr proxyRoute) *LancacheRoute {
	return &LancacheRoute{
		storageService: ss,
		targetService:  ts,
		proxyRoute:     pr,
	}
}

func (r *LancacheRoute) Handler(w http.ResponseWriter, req *http.Request) bool {
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	if req.Method != http.MethodGet || isLocalhost(req) {
		return false
	}

	t, err := r.targetService.Get(ctx, req.Host, req.UserAgent(), req.URL.Path)
	if err != nil {
		return false
	}

	pathname := req.URL.Path
	basePath := t.Code + pathname

	storage, err := r.storageService.Get(ctx, basePath)
	if err != nil {
		storage = r.storageService.Create(basePath, t.Id)
	}

	if storage.IsSuccess() {
		sf := r.storageService.CreateFile(storage, false)
		defer sf.Close()

		w.Header().Add("X-Lancache-Status", "HIT")
		io.Copy(w, sf)

		return true
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

func isLocalhost(req *http.Request) bool {
	host := req.Host
	re := regexp.MustCompile(`^127\.0\.0\.1|localhost`)
	return re.MatchString(host)
}
