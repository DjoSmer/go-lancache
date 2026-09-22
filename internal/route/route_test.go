package route_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"go-lancache/internal/route"

	"github.com/stretchr/testify/require"
)

type R1 struct {
	handlerCalled chan int
}

func (r *R1) Handler(w http.ResponseWriter, req *http.Request) bool {
	<-r.handlerCalled
	return false
}

type R2 struct {
	handlerCalled chan int
}

func (r *R2) Handler(w http.ResponseWriter, req *http.Request) bool {
	<-r.handlerCalled
	return false
}

type R3 struct{}

func (r *R3) Handler(w http.ResponseWriter, req *http.Request) bool {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "hello")
	return true
}

func Test_HandlerCalled(t *testing.T) {
	route := route.NewRoute()
	handlerCalled := make(chan int, 6)

	route.AddRoute(&R1{
		handlerCalled: handlerCalled,
	})
	route.AddRoute(&R2{
		handlerCalled: handlerCalled,
	})

	var wg sync.WaitGroup
	wg.Add(3)

	for i := range 3 {
		handlerCalled <- i + 1
		handlerCalled <- i + 1

		go func() {
			defer wg.Done()
			req := &http.Request{URL: &url.URL{Path: "/game/patch.bin"}}
			rec := httptest.NewRecorder()
			route.ServeHTTP(rec, req)
		}()
	}

	wg.Wait()
}

func Test_NotFound(t *testing.T) {
	route := route.NewRoute()
	handlerCalled := make(chan int, 2)
	handlerCalled <- 1
	handlerCalled <- 2

	route.AddRoute(&R1{
		handlerCalled: handlerCalled,
	})
	route.AddRoute(&R2{
		handlerCalled: handlerCalled,
	})

	req := &http.Request{URL: &url.URL{Path: "/game/patch.bin"}}
	rec := httptest.NewRecorder()
	route.ServeHTTP(rec, req)

	resp := rec.Result()
	require.Equal(t, 404, resp.StatusCode)
}

func Test_If_found_stopped_finding(t *testing.T) {
	route := route.NewRoute()
	handlerCalled := make(chan int, 2)
	handlerCalled <- 1
	handlerCalled <- 2

	route.AddRoute(&R1{
		handlerCalled: handlerCalled,
	})
	route.AddRoute(&R3{})
	route.AddRoute(&R2{
		handlerCalled: handlerCalled,
	})

	req := &http.Request{URL: &url.URL{Path: "/game/patch.bin"}}
	rec := httptest.NewRecorder()
	route.ServeHTTP(rec, req)

	close(handlerCalled)

	require.Equal(t, 2, <-handlerCalled)
	resp := rec.Result()
	result, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hello", string(result))
}
