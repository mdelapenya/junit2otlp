package main

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
)

type TestProxy struct {
	server    *httptest.Server
	targetURL *url.URL
	proxy     *httputil.ReverseProxy
}

func NewTestProxy(targetURL string) (*TestProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	tp := &TestProxy{
		targetURL: target,
	}
	tp.proxy = httputil.NewSingleHostReverseProxy(target)

	originalDirector := tp.proxy.Director
	tp.proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Del("Authorization")
		req.Host = target.Host
	}
	tp.server = httptest.NewServer(tp.proxy)
	return tp, nil
}

func (tp *TestProxy) Close() {
	if tp.server != nil {
		tp.server.Close()
	}
}

func (tp *TestProxy) URL() string {
	return tp.server.URL
}
