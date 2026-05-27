// Package sqlite_test contém testes de integração que sobem um banco SQLite
// real (:memory:) e testam o repositório de ponta a ponta, sem mocks.
package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/FranciscoHonorat/mundo-invest/internal/adapters/sqlite"
	"github.com/FranciscoHonorat/mundo-invest/internal/core/domain"
)

// newTestDB cria um banco SQLite em memória com as tabelas migradas.
func newTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.NewDB(":memory:")
	if err != nil {
		t.Fatalf("falha ao criar banco de teste: %v", err)
	}
	return db
}

// ---------------------------------------------------------------------------
// ClienteRepository — integração
// ---------------------------------------------------------------------------

func TestIntegration_Salvar_BuscarPorEmail(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	cliente, _ := domain.NewCliente("João Silva", "joao@example.com", "Abertura de conta", 300_000)

	// Salvar
	salvo, err := db.Salvar(ctx, cliente)
	if err != nil {
		t.Fatalf("Salvar: não esperava erro, obteve: %v", err)
	}
	if salvo.ID == 0 {
		t.Error("Salvar: ID deve ser preenchido após inserção")
	}

	// Buscar pelo e-mail
	encontrado, err := db.BuscarPorEmail(ctx, "joao@example.com")
	if err != nil {
		t.Fatalf("BuscarPorEmail: não esperava erro, obteve: %v", err)
	}
	if encontrado.Nome != "João Silva" {
		t.Errorf("Nome esperado 'João Silva', obteve '%s'", encontrado.Nome)
	}
	if encontrado.Status != domain.StatusAguardandoAnalise {
		t.Errorf("Status inicial esperado '%s', obteve '%s'", domain.StatusAguardandoAnalise, encontrado.Status)
	}
}

func TestIntegration_BuscarPorEmail_NaoEncontrado(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_, err := db.BuscarPorEmail(ctx, "inexistente@example.com")
	if err != domain.ErrClienteNaoEncontrado {
		t.Errorf("esperava ErrClienteNaoEncontrado, obteve: %v", err)
	}
}

func TestIntegration_Atualizar_StatusEPrioridade(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	cliente, _ := domain.NewCliente("Maria Souza", "maria@example.com", "Resgate", 250_000)
	salvo, _ := db.Salvar(ctx, cliente)

	// Atualizar status e prioridade
	salvo.Status = domain.StatusProcessado
	salvo.Prioridade = domain.PrioridadeAlta
	if err := db.Atualizar(ctx, salvo); err != nil {
		t.Fatalf("Atualizar: não esperava erro, obteve: %v", err)
	}

	// Verificar atualização
	atualizado, _ := db.BuscarPorEmail(ctx, "maria@example.com")
	if atualizado.Status != domain.StatusProcessado {
		t.Errorf("Status esperado '%s', obteve '%s'", domain.StatusProcessado, atualizado.Status)
	}
	if atualizado.Prioridade != domain.PrioridadeAlta {
		t.Errorf("Prioridade esperada '%s', obteve '%s'", domain.PrioridadeAlta, atualizado.Prioridade)
	}
}

func TestIntegration_EmailUnico_NaoPermiteDuplicata(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	c1, _ := domain.NewCliente("A", "dup@example.com", "Tipo", 100_000)
	c2, _ := domain.NewCliente("B", "dup@example.com", "Tipo", 200_000)

	if _, err := db.Salvar(ctx, c1); err != nil {
		t.Fatalf("primeiro Salvar: erro inesperado: %v", err)
	}
	if _, err := db.Salvar(ctx, c2); err == nil {
		t.Error("segundo Salvar com mesmo e-mail deveria retornar erro de constraint")
	}
}

// ---------------------------------------------------------------------------
// WebhookEventRepository — integração (idempotência)
// ---------------------------------------------------------------------------

func TestIntegration_EventoJaProcessado_False(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	jaProcessado, err := db.EventoJaProcessado(ctx, "evt_novo")
	if err != nil {
		t.Fatalf("EventoJaProcessado: erro inesperado: %v", err)
	}
	if jaProcessado {
		t.Error("evento novo não deveria estar registrado")
	}
}

func TestIntegration_RegistrarEvento_Idempotencia(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	event := &domain.WebhookEvent{
		EventID:      "evt_idem_123",
		CardID:       "card_456",
		ClienteEmail: "cliente@example.com",
		Timestamp:    time.Now(),
	}

	// Primeiro registro
	if err := db.RegistrarEvento(ctx, event); err != nil {
		t.Fatalf("RegistrarEvento (1ª vez): erro inesperado: %v", err)
	}

	// Verificar que está registrado
	jaProcessado, err := db.EventoJaProcessado(ctx, "evt_idem_123")
	if err != nil {
		t.Fatalf("EventoJaProcessado: erro inesperado: %v", err)
	}
	if !jaProcessado {
		t.Error("evento deveria estar registrado após RegistrarEvento")
	}

	// Segunda tentativa de registro do mesmo event_id deve falhar (UNIQUE constraint)
	if err := db.RegistrarEvento(ctx, event); err == nil {
		t.Error("segundo RegistrarEvento com mesmo event_id deveria retornar erro de constraint")
	}
}

// ---------------------------------------------------------------------------
// Fluxo completo de integração (end-to-end via repositório)
// ---------------------------------------------------------------------------

func TestIntegration_FluxoCompleto_CriarEProcessarWebhook(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	// 1. Criar cliente
	cliente, _ := domain.NewCliente("Pedro Lima", "pedro@example.com", "Portabilidade", 500_000)
	salvo, err := db.Salvar(ctx, cliente)
	if err != nil {
		t.Fatalf("Salvar: %v", err)
	}

	// 2. Buscar e verificar status inicial
	encontrado, _ := db.BuscarPorEmail(ctx, "pedro@example.com")
	if encontrado.Status != domain.StatusAguardandoAnalise {
		t.Errorf("status inicial incorreto: %s", encontrado.Status)
	}

	// 3. Registrar evento de webhook
	event := &domain.WebhookEvent{
		EventID: "evt_pedro_001", CardID: "card_999",
		ClienteEmail: "pedro@example.com", Timestamp: time.Now(),
	}
	if err := db.RegistrarEvento(ctx, event); err != nil {
		t.Fatalf("RegistrarEvento: %v", err)
	}

	// 4. Atualizar status e prioridade
	salvo.Status = domain.StatusProcessado
	salvo.Prioridade = domain.PrioridadeAlta // 500k >= 200k
	if err := db.Atualizar(ctx, salvo); err != nil {
		t.Fatalf("Atualizar: %v", err)
	}

	// 5. Verificar estado final
	final, _ := db.BuscarPorEmail(ctx, "pedro@example.com")
	if final.Status != domain.StatusProcessado {
		t.Errorf("status final esperado 'Processado', obteve '%s'", final.Status)
	}
	if final.Prioridade != domain.PrioridadeAlta {
		t.Errorf("prioridade esperada 'prioridade_alta', obteve '%s'", final.Prioridade)
	}

	// 6. Confirmar idempotência — evento já registrado
	jaProcessado, _ := db.EventoJaProcessado(ctx, "evt_pedro_001")
	if !jaProcessado {
		t.Error("evento deveria estar marcado como processado")
	}
}
