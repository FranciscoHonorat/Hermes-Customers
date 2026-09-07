package pipefy

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
)

func TestNewClient_SemToken_OperaEmModoSimulado(t *testing.T) {
	os.Unsetenv("PIPEFY_API_TOKEN")
	c := NewClient()
	if !c.simulated {
		t.Fatal("esperava cliente em modo simulado quando PIPEFY_API_TOKEN não está definido")
	}
}

func TestCriarCard_ModoSimulado_RetornaCardIDPrevisivel(t *testing.T) {
	c := NewClient()
	cliente := &domain.Cliente{ID: 42, Nome: "João Silva", Email: "joao@example.com"}

	cardID, err := c.CriarCard(context.Background(), cliente)
	if err != nil {
		t.Fatalf("não esperava erro, obteve: %v", err)
	}
	if !strings.Contains(cardID, "42") {
		t.Errorf("esperava card_id contendo o ID do cliente, obteve: %s", cardID)
	}
}

func TestAtualizarCard_ModoSimulado_NaoRetornaErro(t *testing.T) {
	c := NewClient()
	err := c.AtualizarCard(context.Background(), "card_456", domain.StatusProcessado, domain.PrioridadeAlta)
	if err != nil {
		t.Fatalf("não esperava erro em modo simulado, obteve: %v", err)
	}
}
