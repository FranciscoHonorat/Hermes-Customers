package service

import (
	"context"
	"testing"
	"time"

	"github.com/FranciscoHonorat/mundo-invest/internal/core/domain"
	"github.com/FranciscoHonorat/mundo-invest/internal/core/ports/output"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type MockClienteRepository struct {
	SalvarFn         func(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error)
	BuscarPorEmailFn func(ctx context.Context, email string) (*domain.Cliente, error)
	AtualizarFn      func(ctx context.Context, c *domain.Cliente) error
}

func (m *MockClienteRepository) Salvar(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
	if m.SalvarFn != nil {
		return m.SalvarFn(ctx, c)
	}
	c.ID = 1
	return c, nil
}

func (m *MockClienteRepository) BuscarPorEmail(ctx context.Context, email string) (*domain.Cliente, error) {
	if m.BuscarPorEmailFn != nil {
		return m.BuscarPorEmailFn(ctx, email)
	}
	return nil, domain.ErrClienteNaoEncontrado
}

func (m *MockClienteRepository) Atualizar(ctx context.Context, c *domain.Cliente) error {
	if m.AtualizarFn != nil {
		return m.AtualizarFn(ctx, c)
	}
	return nil
}

// Garante que MockClienteRepository implementa a interface
var _ output.ClienteRepository = (*MockClienteRepository)(nil)

// ---------------------------------------------------------------------------

type MockWebhookEventRepository struct {
	EventoJaProcessadoFn func(ctx context.Context, eventID string) (bool, error)
	RegistrarEventoFn    func(ctx context.Context, event *domain.WebhookEvent) error
}

func (m *MockWebhookEventRepository) EventoJaProcessado(ctx context.Context, eventID string) (bool, error) {
	if m.EventoJaProcessadoFn != nil {
		return m.EventoJaProcessadoFn(ctx, eventID)
	}
	return false, nil
}

func (m *MockWebhookEventRepository) RegistrarEvento(ctx context.Context, event *domain.WebhookEvent) error {
	if m.RegistrarEventoFn != nil {
		return m.RegistrarEventoFn(ctx, event)
	}
	return nil
}

var _ output.WebhookEventRepository = (*MockWebhookEventRepository)(nil)

// ---------------------------------------------------------------------------

type MockPipefyClient struct {
	CriarCardFn     func(ctx context.Context, c *domain.Cliente) (string, error)
	AtualizarCardFn func(ctx context.Context, cardID string, status domain.StatusCliente, p domain.Prioridade) error
}

func (m *MockPipefyClient) CriarCard(ctx context.Context, c *domain.Cliente) (string, error) {
	if m.CriarCardFn != nil {
		return m.CriarCardFn(ctx, c)
	}
	return "card_mock_123", nil
}

func (m *MockPipefyClient) AtualizarCard(ctx context.Context, cardID string, status domain.StatusCliente, p domain.Prioridade) error {
	if m.AtualizarCardFn != nil {
		return m.AtualizarCardFn(ctx, cardID, status, p)
	}
	return nil
}

var _ output.PipefyClient = (*MockPipefyClient)(nil)

// ---------------------------------------------------------------------------

type MockEventPublisher struct {
	PublicarFn func(ctx context.Context, event output.ClienteCreatedEvent) error
}

func (m *MockEventPublisher) PublicarClienteCriado(ctx context.Context, event output.ClienteCreatedEvent) error {
	if m.PublicarFn != nil {
		return m.PublicarFn(ctx, event)
	}
	return nil
}

var _ output.EventPublisher = (*MockEventPublisher)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newService(cr output.ClienteRepository, wr output.WebhookEventRepository, pc output.PipefyClient) *ClienteService {
	return NewClienteService(cr, wr, pc, &MockEventPublisher{})
}

func clienteValido() *domain.Cliente {
	c, _ := domain.NewCliente("João Silva", "joao@example.com", "Atualização cadastral", 250_000)
	return c
}

func webhookEvent() *domain.WebhookEvent {
	return &domain.WebhookEvent{
		EventID:      "evt_123",
		CardID:       "card_456",
		ClienteEmail: "joao@example.com",
		Timestamp:    time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Teste 1: Criação de cliente com payload válido e salvamento no banco
// ---------------------------------------------------------------------------

func TestCriarCliente_PayloadValido_SalvaNoBanco(t *testing.T) {
	salvou := false
	clienteRepo := &MockClienteRepository{
		SalvarFn: func(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
			salvou = true
			c.ID = 42
			if c.Status != domain.StatusAguardandoAnalise {
				t.Errorf("status inicial esperado '%s', obteve '%s'", domain.StatusAguardandoAnalise, c.Status)
			}
			return c, nil
		},
	}

	svc := newService(clienteRepo, &MockWebhookEventRepository{}, &MockPipefyClient{})
	criado, err := svc.CriarCliente(context.Background(), clienteValido())

	if err != nil {
		t.Fatalf("não esperava erro, obteve: %v", err)
	}
	if !salvou {
		t.Error("esperava que o cliente fosse salvo no banco")
	}
	if criado.ID != 42 {
		t.Errorf("ID esperado 42, obteve %d", criado.ID)
	}
}

func TestCriarCliente_EmailInvalido_RetornaErro(t *testing.T) {
	c := &domain.Cliente{
		Nome:            "João",
		Email:           "email-invalido",
		TipoSolicitacao: "Abertura de conta",
		ValorPatrimonio: 100_000,
		Status:          domain.StatusAguardandoAnalise,
	}
	svc := newService(&MockClienteRepository{}, &MockWebhookEventRepository{}, &MockPipefyClient{})
	_, err := svc.CriarCliente(context.Background(), c)
	if err != domain.ErrEmailInvalido {
		t.Errorf("esperava ErrEmailInvalido, obteve: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Teste 2: Webhook aplica regra de prioridade correta com base no patrimônio
// ---------------------------------------------------------------------------

func TestProcessarWebhook_PatrimonioAlto_PrioridadeAlta(t *testing.T) {
	var prioridadeDefinida domain.Prioridade

	clienteRepo := &MockClienteRepository{
		BuscarPorEmailFn: func(ctx context.Context, email string) (*domain.Cliente, error) {
			return &domain.Cliente{
				ID:              1,
				Nome:            "João Silva",
				Email:           email,
				ValorPatrimonio: 250_000, // >= 200k → prioridade_alta
				Status:          domain.StatusAguardandoAnalise,
			}, nil
		},
		AtualizarFn: func(ctx context.Context, c *domain.Cliente) error {
			prioridadeDefinida = c.Prioridade
			if c.Status != domain.StatusProcessado {
				t.Errorf("status esperado '%s', obteve '%s'", domain.StatusProcessado, c.Status)
			}
			return nil
		},
	}

	svc := newService(clienteRepo, &MockWebhookEventRepository{}, &MockPipefyClient{})
	err := svc.ProcessarWebhook(context.Background(), webhookEvent())
	if err != nil {
		t.Fatalf("não esperava erro, obteve: %v", err)
	}
	if prioridadeDefinida != domain.PrioridadeAlta {
		t.Errorf("esperava prioridade_alta, obteve: %s", prioridadeDefinida)
	}
}

func TestProcessarWebhook_PatrimonioNormal_PrioridadeNormal(t *testing.T) {
	var prioridadeDefinida domain.Prioridade

	clienteRepo := &MockClienteRepository{
		BuscarPorEmailFn: func(ctx context.Context, email string) (*domain.Cliente, error) {
			return &domain.Cliente{
				ID:              2,
				Nome:            "Maria Souza",
				Email:           email,
				ValorPatrimonio: 150_000, // < 200k → prioridade_normal
				Status:          domain.StatusAguardandoAnalise,
			}, nil
		},
		AtualizarFn: func(ctx context.Context, c *domain.Cliente) error {
			prioridadeDefinida = c.Prioridade
			return nil
		},
	}

	event := &domain.WebhookEvent{
		EventID: "evt_456", CardID: "card_789",
		ClienteEmail: "maria@example.com", Timestamp: time.Now(),
	}

	svc := newService(clienteRepo, &MockWebhookEventRepository{}, &MockPipefyClient{})
	err := svc.ProcessarWebhook(context.Background(), event)
	if err != nil {
		t.Fatalf("não esperava erro, obteve: %v", err)
	}
	if prioridadeDefinida != domain.PrioridadeNormal {
		t.Errorf("esperava prioridade_normal, obteve: %s", prioridadeDefinida)
	}
}

// ---------------------------------------------------------------------------
// Teste 3: Bloqueio de processamento para event_id duplicado
// ---------------------------------------------------------------------------

func TestProcessarWebhook_EventIDDuplicado_Bloqueado(t *testing.T) {
	webhookRepo := &MockWebhookEventRepository{
		EventoJaProcessadoFn: func(ctx context.Context, eventID string) (bool, error) {
			return true, nil // simula evento já registrado
		},
	}

	svc := newService(&MockClienteRepository{}, webhookRepo, &MockPipefyClient{})
	err := svc.ProcessarWebhook(context.Background(), webhookEvent())

	if err != domain.ErrEventoDuplicado {
		t.Errorf("esperava ErrEventoDuplicado, obteve: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Teste 4 (event-driven): Evento é publicado no RabbitMQ ao criar cliente
// ---------------------------------------------------------------------------

func TestCriarCliente_PublicaEventoRabbitMQ(t *testing.T) {
	publicou := false
	var emailPublicado string

	publisher := &MockEventPublisher{
		PublicarFn: func(ctx context.Context, event output.ClienteCreatedEvent) error {
			publicou = true
			emailPublicado = event.Email
			return nil
		},
	}

	clienteRepo := &MockClienteRepository{
		SalvarFn: func(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
			c.ID = 7
			return c, nil
		},
	}

	svc := NewClienteService(clienteRepo, &MockWebhookEventRepository{}, &MockPipefyClient{}, publisher)
	_, err := svc.CriarCliente(context.Background(), clienteValido())

	if err != nil {
		t.Fatalf("não esperava erro, obteve: %v", err)
	}
	if !publicou {
		t.Error("esperava que o evento fosse publicado no RabbitMQ")
	}
	if emailPublicado != "joao@example.com" {
		t.Errorf("email publicado esperado 'joao@example.com', obteve '%s'", emailPublicado)
	}
}
