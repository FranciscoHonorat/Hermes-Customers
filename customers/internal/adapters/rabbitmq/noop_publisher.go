package rabbitmq

import (
	"context"
	"log/slog"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/ports/output"
)

// NoopPublisher é usado quando RABBITMQ_URI não está configurado.
// Loga o evento sem enviar para nenhuma fila — garante degradação graciosa.
type NoopPublisher struct{}

func (n *NoopPublisher) PublicarClienteCriado(ctx context.Context, event output.ClienteCreatedEvent) error {
	slog.Info("[RABBITMQ SIMULADO] evento não publicado (sem conexão)", slog.String("email", event.Email))
	return nil
}

var _ output.EventPublisher = (*NoopPublisher)(nil)
