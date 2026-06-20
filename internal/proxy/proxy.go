package proxy

import (
	"context"
	"net/http"
)

type ProxyHooks interface {
	BeforeRequest(p *Proxy)
	InterceptResponse(p *Proxy)
	AfterResponse(p *Proxy, err error)
}

type Proxy struct {
	Ctx       context.Context
	Req       *http.Request
	Resp      *http.Response
	RequestId int64

	Hooks ProxyHooks
}
