package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// RequestIDHeader é lido do api-gateway quando presente, para correlacionar
// logs entre os dois serviços; gerado aqui quando o serviço é chamado direto.
const RequestIDHeader = "X-Request-ID"

// RequestID garante que toda requisição tenha um identificador único,
// devolvido na resposta e disponível em c.GetString("request_id") para logs.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		c.Writer.Header().Set(RequestIDHeader, id)
		c.Set("request_id", id)
		c.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "req-unavailable"
	}
	return hex.EncodeToString(b)
}
