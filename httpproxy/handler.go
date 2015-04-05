package httpproxy

import (
	"github.com/golang/glog"
	"github.com/phuslu/goproxy/httpproxy/filters"
	"io"
	"net"
	"net/http"
)

type Handler struct {
	http.Handler
	Listener        net.Listener
	Transport       *http.Transport
	RequestFilters  []filters.RequestFilter
	FetchFilters    []filters.FetchFilter
	ResponseFilters []filters.ResponseFilter
}

func (h Handler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Enable transport http proxy
	if req.Method != "CONNECT" && !req.URL.IsAbs() {
		if req.TLS != nil {
			req.URL.Scheme = "https"
			if req.Host != "" {
				req.URL.Host = req.Host
			} else {
				req.URL.Host = req.TLS.ServerName
			}
		} else {
			req.URL.Scheme = "http"
			req.URL.Host = req.Host
		}
	}

	// Prepare filter.Context
	var err error
	ctx := &filters.Context{
		"__listener__":  h.Listener,
		"__transport__": h.Transport,
		"__rw__":        rw,
	}

	// Filter Request
	for _, f := range h.RequestFilters {
		ctx, req, err = f.Request(ctx, req)
		if err != nil {
			glog.Infof("ServeHTTP %#v error: %v", f, err)
			return
		}
		if req == nil {
			return
		}
	}

	// Filter Request -> Response
	var resp *http.Response
	for _, f := range h.FetchFilters {
		ctx, resp, err = f.Fetch(ctx, req)
		if err != nil {
			glog.Infof("ServeHTTP %#v error: %v", f, err)
			return
		}
		if resp != nil {
			resp.Request = req
			break
		}
	}

	// Filter Response
	for _, f := range h.ResponseFilters {
		ctx, resp, err = f.Response(ctx, resp)
		if err != nil {
			glog.Infof("ServeHTTP %#v error: %v", f, err)
			return
		}
		if resp == nil {
			return
		}
	}

	for key, values := range resp.Header {
		for _, value := range values {
			rw.Header().Add(key, value)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	io.Copy(rw, resp.Body)
}
