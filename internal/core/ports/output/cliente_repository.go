package output

import (
	"context"

	"github.com/FranciscoHonorat/mundo-invest/internal/core/domain"
)

// ClienteRepository define as operações de persistência para clientes.
type ClienteRepository interface {
	// Salvar persiste um novo cliente no banco de dados.
	Salvar(ctx context.Context, cliente *domain.Cliente) (*domain.Cliente, error)

	// BuscarPorEmail retorna o cliente com o e-mail informado.
	BuscarPorEmail(ctx context.Context, email string) (*domain.Cliente, error)

	// Atualizar atualiza status e prioridade do cliente.
	Atualizar(ctx context.Context, cliente *domain.Cliente) error
}

// WebhookEventRepository define as operações de persistência para eventos de webhook.
type WebhookEventRepository interface {
	// EventoJaProcessado verifica se o event_id já foi registrado (idempotência).
	EventoJaProcessado(ctx context.Context, eventID string) (bool, error)

	// RegistrarEvento persiste o evento para controle de idempotência.
	RegistrarEvento(ctx context.Context, event *domain.WebhookEvent) error
}
