package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PipefySignatureHeader é o cabeçalho onde o Pipefy envia a assinatura
// HMAC-SHA256 (hex) do corpo bruto da requisição de webhook.
const PipefySignatureHeader = "X-Pipefy-Signature"

// RequireWebhookSignature valida que o corpo da requisição foi assinado com
// secret via HMAC-SHA256. Quando secret está vazio, o webhook é aceito sem
// verificação — mesma política de degradação graciosa dos demais adapters
// opcionais deste serviço, mas deixa o handler exposto a payloads forjados
// (ver PIPEFY_WEBHOOK_SECRET no README/.env.example).
func RequireWebhookSignature(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "não foi possível ler o corpo da requisição"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := hex.EncodeToString(mac.Sum(nil))

		got := c.GetHeader(PipefySignatureHeader)
		if got == "" || !hmac.Equal([]byte(got), []byte(expected)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "assinatura de webhook inválida"})
			return
		}
		c.Next()
	}
}
