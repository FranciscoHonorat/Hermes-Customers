package main

import (
	"log/slog"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"

	"api-gateway/internal/adapters/proxy"
	"api-gateway/internal/handlers"
)

// @title           Hermes API Gateway
// @version         1.0
// @description     Ponto de entrada único da Hermes, roteando requisições para os serviços internos.
// @termsOfService  http://swagger.io/terms/
// @contact.name    API Support
// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT
// @host            localhost:8000
// @BasePath        /api/v1
// @schemes         http https
func main() {
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

	r := gin.Default()

	r.GET("/health", handlers.HealthHandler)

	customers := r.Group("/api/v1/customers")
	customers.Any("", gin.WrapH(customersProxy))
	customers.Any("/*path", gin.WrapH(customersProxy))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	slog.Info("api-gateway iniciado", slog.String("port", port), slog.String("customers_url", customersURL))
	if err := r.Run(":" + port); err != nil {
		slog.Error("falha ao iniciar servidor", slog.Any("error", err))
		os.Exit(1)
	}
}
