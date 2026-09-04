package output

import (
	"context"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
)

// ClienteCreatedEvent é o payload publicado na fila quando um cliente é criado.
type ClienteCreatedEvent struct {
	ClienteID       int64                `json:"cliente_id"`
	Nome            string               `json:"nome"`
	Email           string               `json:"email"`
	TipoSolicitacao string               `json:"tipo_solicitacao"`
	ValorPatrimonio float64              `json:"valor_patrimonio"`
	Status          domain.StatusCliente `json:"status"`
}

// EventPublisher define a interface para publicação de eventos assíncronos.
type EventPublisher interface {
	// PublicarClienteCriado envia um evento de criação de cliente para a fila.
	PublicarClienteCriado(ctx context.Context, event ClienteCreatedEvent) error
}
