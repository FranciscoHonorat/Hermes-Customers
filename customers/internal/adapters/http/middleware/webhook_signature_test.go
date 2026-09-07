package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestRequireWebhookSignature_SecretVazio_NaoBloqueia(t *testing.T) {
	r := newTestRouter(RequireWebhookSignature(""))
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(`{"a":1}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200 sem secret configurado, obteve %d", rec.Code)
	}
}

func TestRequireWebhookSignature_SemAssinatura_Retorna401(t *testing.T) {
	r := newTestRouter(RequireWebhookSignature("segredo"))
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(`{"a":1}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperava 401 sem assinatura, obteve %d", rec.Code)
	}
}

func TestRequireWebhookSignature_AssinaturaValida_Retorna200(t *testing.T) {
	body := `{"a":1}`
	r := newTestRouter(RequireWebhookSignature("segredo"))
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set(PipefySignatureHeader, sign("segredo", body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200 com assinatura válida, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestRequireWebhookSignature_AssinaturaInvalida_Retorna401(t *testing.T) {
	body := `{"a":1}`
	r := newTestRouter(RequireWebhookSignature("segredo"))
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(body))
	req.Header.Set(PipefySignatureHeader, sign("outro-segredo", body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperava 401 com assinatura inválida, obteve %d", rec.Code)
	}
}

func newTestRouter(mw gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.POST("/webhook", mw, newOKHandler())
	return r
}
