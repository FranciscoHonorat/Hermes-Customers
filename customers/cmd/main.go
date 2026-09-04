package main

import (
	"log/slog"
	"os"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/http/handlers"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/pipefy"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/rabbitmq"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/adapters/sqlite"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/ports/output"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/service"
	"github.com/gin-gonic/gin"
)

func main() {
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

	// --- Router ---
	r := gin.Default()

	r.GET("/health", handlers.HealthHandler)
	r.POST("/clientes", clienteHandler.CriarCliente)
	r.POST("/webhooks/pipefy/card-updated", webhookHandler.CardUpdated)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("servidor iniciado", slog.String("port", port))
	if err := r.Run(":" + port); err != nil {
		slog.Error("falha ao iniciar servidor", slog.Any("error", err))
		os.Exit(1)
	}
}
