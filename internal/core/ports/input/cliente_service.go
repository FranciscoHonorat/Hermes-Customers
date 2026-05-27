package input

import (
	"context"

	"github.com/FranciscoHonorat/mundo-invest/internal/core/domain"
)

// ClienteService define as operações de negócio expostas para os handlers HTTP.
type ClienteService interface {
	// CriarCliente valida, persiste o cliente e mapeia o card no Pipefy.
	CriarCliente(ctx context.Context, cliente *domain.Cliente) (*domain.Cliente, error)

	// ProcessarWebhook aplica idempotência, calcula prioridade e atualiza o card no Pipefy.
	ProcessarWebhook(ctx context.Context, event *domain.WebhookEvent) error
}
