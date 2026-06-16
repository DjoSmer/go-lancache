package proxy

import (
	"context"
	"net/http"
)

type InterceptResponseFunc func(p *Proxy)

type Proxy struct {
	Ctx       context.Context
	Req       *http.Request
	Resp      *http.Response
	RequestId int64
	Intercept InterceptResponseFunc
}
