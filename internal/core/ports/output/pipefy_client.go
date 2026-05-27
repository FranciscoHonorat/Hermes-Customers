package output

import (
	"context"

	"github.com/FranciscoHonorat/mundo-invest/internal/core/domain"
)

// PipefyClient define as operações de integração com o Pipefy via GraphQL.
type PipefyClient interface {
	// CriarCard mapeia um novo cliente como card no Pipefy (mutation createCard).
	CriarCard(ctx context.Context, cliente *domain.Cliente) (cardID string, err error)

	// AtualizarCard atualiza status e prioridade de um card existente no Pipefy
	// (mutation updateCardField).
	AtualizarCard(ctx context.Context, cardID string, status domain.StatusCliente, prioridade domain.Prioridade) error
}
