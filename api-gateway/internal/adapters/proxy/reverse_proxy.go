package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewReverseProxy cria um reverse proxy HTTP para target, removendo
// stripPrefix do path da requisição antes de encaminhá-la.
func NewReverseProxy(target *url.URL, stripPrefix string) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)

	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, stripPrefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
		director(req)
	}

	return proxy
}
