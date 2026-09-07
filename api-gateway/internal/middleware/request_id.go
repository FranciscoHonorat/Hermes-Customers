package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// RequestIDHeader é propagado do cliente (se presente) ou gerado aqui, e
// repassado ao serviço customers para correlacionar logs entre os dois.
const RequestIDHeader = "X-Request-ID"

// RequestID garante que toda requisição tenha um identificador único,
// devolvido na resposta e disponível em c.GetString("request_id") para logs.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = newRequestID()
			c.Request.Header.Set(RequestIDHeader, id)
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
