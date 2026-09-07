// Package middleware contém os middlewares HTTP do serviço customers:
// autenticação por API key, verificação de assinatura de webhook, request-id
// e métricas Prometheus.
package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireAPIKey protege endpoints de escrita com um token Bearer estático,
// comparado em tempo constante. Quando token está vazio, o middleware não
// bloqueia nada (mesma política de degradação graciosa usada pelos adapters
// Pipefy/RabbitMQ) — mas registra um aviso no boot (ver cmd/main.go), já que
// rodar sem token em produção é uma escolha explícita, não o padrão desejado.
func RequireAPIKey(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}

		const prefix = "Bearer "
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "não autorizado"})
			return
		}

		provided := strings.TrimPrefix(auth, prefix)
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "não autorizado"})
			return
		}
		c.Next()
	}
}
