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

func newOKHandler() gin.HandlerFunc {
	return func(c *gin.Context) { c.Status(http.StatusOK) }
}

func TestRequireAPIKey_TokenVazio_NaoBloqueia(t *testing.T) {
	r := gin.New()
	r.GET("/protegido", RequireAPIKey(""), newOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200 sem token configurado, obteve %d", rec.Code)
	}
}

func TestRequireAPIKey_SemHeader_Retorna401(t *testing.T) {
	r := gin.New()
	r.GET("/protegido", RequireAPIKey("segredo"), newOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperava 401 sem header Authorization, obteve %d", rec.Code)
	}
}

func TestRequireAPIKey_TokenErrado_Retorna401(t *testing.T) {
	r := gin.New()
	r.GET("/protegido", RequireAPIKey("segredo"), newOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	req.Header.Set("Authorization", "Bearer errado")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperava 401 com token incorreto, obteve %d", rec.Code)
	}
}

func TestRequireAPIKey_TokenCorreto_Retorna200(t *testing.T) {
	r := gin.New()
	r.GET("/protegido", RequireAPIKey("segredo"), newOKHandler())

	req := httptest.NewRequest(http.MethodGet, "/protegido", nil)
	req.Header.Set("Authorization", "Bearer segredo")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200 com token correto, obteve %d", rec.Code)
	}
}
