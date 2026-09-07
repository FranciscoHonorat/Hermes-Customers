package rabbitmq

import (
	"context"
	"testing"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/ports/output"
)

func TestNoopPublisher_PublicarClienteCriado_NuncaFalha(t *testing.T) {
	p := &NoopPublisher{}
	event := output.ClienteCreatedEvent{
		ClienteID: 1,
		Nome:      "João Silva",
		Email:     "joao@example.com",
		Status:    domain.StatusAguardandoAnalise,
	}

	if err := p.PublicarClienteCriado(context.Background(), event); err != nil {
		t.Fatalf("NoopPublisher não deveria falhar, obteve: %v", err)
	}
}
