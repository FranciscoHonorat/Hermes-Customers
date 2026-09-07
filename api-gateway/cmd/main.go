package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"api-gateway/internal/adapters/proxy"
	"api-gateway/internal/handlers"
	"api-gateway/internal/middleware"
)

// @title           Mundo Invest API Gateway
// @version         1.0
// @description     Ponto de entrada único da API, roteando requisições para os serviços internos.
// @termsOfService  http://swagger.io/terms/
// @contact.name    API Support
// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT
// @host            localhost:8000
// @BasePath        /api/v1
// @schemes         http https
func main() {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	customersURL := os.Getenv("CUSTOMERS_SERVICE_URL")
	if customersURL == "" {
		customersURL = "http://localhost:8080"
	}

	target, err := url.Parse(customersURL)
	if err != nil {
		slog.Error("CUSTOMERS_SERVICE_URL inválida", slog.String("url", customersURL), slog.Any("error", err))
		os.Exit(1)
	}
	customersProxy := proxy.NewReverseProxy(target, "/api/v1/customers")

	allowedOrigins := splitAndTrim(os.Getenv("ALLOWED_ORIGINS"))
	rateLimitPerSecond := envFloat("RATE_LIMIT_PER_SECOND", 10)
	rateLimitBurst := envFloat("RATE_LIMIT_BURST", 20)
	limiter := middleware.NewRateLimiter(rateLimitPerSecond, rateLimitBurst)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Metrics(), middleware.CORS(allowedOrigins), limiter.Middleware())

	r.GET("/health", handlers.HealthHandler)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	customers := r.Group("/api/v1/customers")
	customers.Any("", gin.WrapH(customersProxy))
	customers.Any("/*path", gin.WrapH(customersProxy))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("api-gateway iniciado", slog.String("port", port), slog.String("customers_url", customersURL))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("falha ao iniciar servidor", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	waitForShutdown(srv)
}

// waitForShutdown bloqueia até SIGINT/SIGTERM e então drena conexões em voo
// antes de encerrar — evita que o Kubernetes derrube requisições no meio de
// um rolling update ou scale down.
func waitForShutdown(srv *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("sinal de encerramento recebido, drenando conexões...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("erro durante shutdown gracioso", slog.Any("error", err))
	} else {
		slog.Info("servidor encerrado com sucesso")
	}
}

func splitAndTrim(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		slog.Warn("valor inválido para variável de ambiente, usando padrão", slog.String("key", key), slog.String("value", v))
		return def
	}
	return f
}
