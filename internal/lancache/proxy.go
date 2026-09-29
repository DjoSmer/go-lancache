package lancache

import (
	"net/http"
)

type proxyRoute interface {
	Handler(w http.ResponseWriter, req *http.Request) bool
}
