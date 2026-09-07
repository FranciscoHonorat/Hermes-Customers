package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/http/handlers"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/http/middleware"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/pipefy"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/rabbitmq"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/sqlite"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/ports/output"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/service"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	// --- Banco de dados SQLite ---
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		dsn = "mundo_invest.db"
	}

	db, err := sqlite.NewDB(dsn)
	if err != nil {
		slog.Error("falha ao inicializar banco SQLite", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("banco SQLite inicializado", slog.String("dsn", dsn))

	// --- Pipefy Client (simulado se sem token) ---
	pipefyClient := pipefy.NewClient()

	// --- RabbitMQ Publisher (noop se sem URI) ---
	var eventPublisher output.EventPublisher
	rabbitmqURI := os.Getenv("RABBITMQ_URI")
	if rabbitmqURI != "" {
		pub, err := rabbitmq.NewPublisher(rabbitmqURI)
		if err != nil {
			slog.Warn("falha ao conectar RabbitMQ — operando sem publicação de eventos", slog.Any("error", err))
			eventPublisher = &rabbitmq.NoopPublisher{}
		} else {
			defer pub.Close()
			eventPublisher = pub
		}
	} else {
		slog.Warn("RABBITMQ_URI não configurado — publicação de eventos desativada")
		eventPublisher = &rabbitmq.NoopPublisher{}
	}

	// --- Service (regras de negócio) ---
	svc := service.NewClienteService(db, db, pipefyClient, eventPublisher)

	// --- Handlers HTTP ---
	clienteHandler := handlers.NewClienteHandler(svc)
	webhookHandler := handlers.NewWebhookHandler(svc)

	// --- Autenticação ---
	apiToken := os.Getenv("API_AUTH_TOKEN")
	if apiToken == "" {
		slog.Warn("API_AUTH_TOKEN não configurado — POST /clientes está sem autenticação")
	}
	webhookSecret := os.Getenv("PIPEFY_WEBHOOK_SECRET")
	if webhookSecret == "" {
		slog.Warn("PIPEFY_WEBHOOK_SECRET não configurado — webhook aceita payloads sem verificação de assinatura")
	}

	// --- Router ---
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Metrics())

	r.GET("/health", handlers.HealthHandler)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	r.POST("/clientes", middleware.RequireAPIKey(apiToken), clienteHandler.CriarCliente)
	r.POST("/webhooks/pipefy/card-updated", middleware.RequireWebhookSignature(webhookSecret), webhookHandler.CardUpdated)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("servidor iniciado", slog.String("port", port))
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
