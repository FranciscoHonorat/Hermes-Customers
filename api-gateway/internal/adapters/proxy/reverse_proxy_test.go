package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewReverseProxy_StripPrefix_RemovePrefixDoPath(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	target, _ := url.Parse(backend.URL)
	proxy := NewReverseProxy(target, "/api/v1/customers")
	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/api/v1/customers/health")
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if receivedPath != "/health" {
		t.Errorf("esperava path '/health' no backend, obteve '%s'", receivedPath)
	}
}

func TestNewReverseProxy_PathVazioAposStrip_VirBarraRaiz(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	target, _ := url.Parse(backend.URL)
	proxy := NewReverseProxy(target, "/api/v1/customers")
	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	resp, err := http.Get(frontend.URL + "/api/v1/customers")
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if receivedPath != "/" {
		t.Errorf("esperava path '/' no backend quando prefixo consome o path inteiro, obteve '%s'", receivedPath)
	}
}

func TestNewReverseProxy_EncaminhaMetodoECorpo(t *testing.T) {
	var receivedMethod, receivedBody string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer backend.Close()

	target, _ := url.Parse(backend.URL)
	proxy := NewReverseProxy(target, "/api/v1/customers")
	frontend := httptest.NewServer(proxy)
	defer frontend.Close()

	payload := `{"cliente_nome":"João"}`
	req, _ := http.NewRequest(http.MethodPost, frontend.URL+"/api/v1/customers/clientes", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("esperava 201 do backend via proxy, obteve %d", resp.StatusCode)
	}
	if receivedMethod != http.MethodPost {
		t.Errorf("esperava método POST encaminhado, obteve %s", receivedMethod)
	}
	if receivedBody != payload {
		t.Errorf("esperava corpo '%s' encaminhado, obteve '%s'", payload, receivedBody)
	}
}
