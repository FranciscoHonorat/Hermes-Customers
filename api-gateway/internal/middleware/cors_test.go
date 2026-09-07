package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newRouterWithOK(mw gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(mw)
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCORS_OrigemNaoPermitida_SemCabecalho(t *testing.T) {
	r := newRouterWithOK(CORS([]string{"https://app.exemplo.com"}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://malicioso.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("esperava sem Access-Control-Allow-Origin para origem não permitida, obteve '%s'", got)
	}
}

func TestCORS_OrigemPermitida_Reflectida(t *testing.T) {
	r := newRouterWithOK(CORS([]string{"https://app.exemplo.com"}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://app.exemplo.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.exemplo.com" {
		t.Errorf("esperava origem refletida no header, obteve '%s'", got)
	}
}

func TestCORS_Preflight_Retorna204(t *testing.T) {
	r := newRouterWithOK(CORS([]string{"*"}))
	req := httptest.NewRequest(http.MethodOptions, "/x", nil)
	req.Header.Set("Origin", "https://qualquer.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("esperava 204 no preflight, obteve %d", rec.Code)
	}
}

func TestRateLimiter_ExcedeuRajada_Retorna429(t *testing.T) {
	limiter := NewRateLimiter(1, 2)
	r := newRouterWithOK(limiter.Middleware())

	var lastCode int
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "203.0.113.1:1234"
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		lastCode = rec.Code
	}

	if lastCode != http.StatusTooManyRequests {
		t.Errorf("esperava 429 após exceder a rajada, obteve %d", lastCode)
	}
}
