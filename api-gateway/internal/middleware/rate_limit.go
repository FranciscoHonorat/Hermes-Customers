package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// visitor rastreia o balde de tokens (token bucket) de um cliente.
type visitor struct {
	tokens float64
	last   time.Time
}

// RateLimiter implementa um limitador por IP usando o algoritmo de token
// bucket, sem dependências externas.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     float64 // tokens repostos por segundo
	burst    float64 // capacidade máxima do balde
}

// NewRateLimiter cria um limitador que permite, em regime permanente,
// ratePerSecond requisições por segundo por IP, com rajadas de até burst.
func NewRateLimiter(ratePerSecond, burst float64) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     ratePerSecond,
		burst:    burst,
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, ok := rl.visitors[key]
	if !ok {
		rl.visitors[key] = &visitor{tokens: rl.burst - 1, last: now}
		return true
	}

	elapsed := now.Sub(v.last).Seconds()
	v.tokens += elapsed * rl.rate
	if v.tokens > rl.burst {
		v.tokens = rl.burst
	}
	v.last = now

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

// Middleware retorna 429 quando o IP do cliente excede a taxa configurada.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "muitas requisições, tente novamente em instantes",
			})
			return
		}
		c.Next()
	}
}
