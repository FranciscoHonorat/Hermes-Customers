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
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.URL.Path = strings.TrimPrefix(r.In.URL.Path, stripPrefix)
			if r.Out.URL.Path == "" {
				r.Out.URL.Path = "/"
			}
		},
	}
}
