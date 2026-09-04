package service

import (
	"context"
	"log/slog"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/ports/output"
)

// ClienteService implementa a interface input.ClienteService com as regras de negócio.
type ClienteService struct {
	clienteRepo    output.ClienteRepository
	webhookRepo    output.WebhookEventRepository
	pipefy         output.PipefyClient
	eventPublisher output.EventPublisher
}

func NewClienteService(
	clienteRepo output.ClienteRepository,
	webhookRepo output.WebhookEventRepository,
	pipefy output.PipefyClient,
	eventPublisher output.EventPublisher,
) *ClienteService {
	return &ClienteService{
		clienteRepo:    clienteRepo,
		webhookRepo:    webhookRepo,
		pipefy:         pipefy,
		eventPublisher: eventPublisher,
	}
}

// CriarCliente valida, persiste o cliente com status "Aguardando Análise",
// aciona a mutation createCard no Pipefy (simulado) e publica evento no RabbitMQ.
func (s *ClienteService) CriarCliente(ctx context.Context, cliente *domain.Cliente) (*domain.Cliente, error) {
	// 1. Validação de domínio
	if err := cliente.Validate(); err != nil {
		return nil, err
	}

	// 2. Persistência local com status inicial
	salvo, err := s.clienteRepo.Salvar(ctx, cliente)
	if err != nil {
		return nil, err
	}

	// 3. Mapeamento Pipefy — mutation createCard (simulado, log em produção seria o envio real)
	cardID, err := s.pipefy.CriarCard(ctx, salvo)
	if err != nil {
		// Não bloqueia o fluxo: loga e segue (política de degradação graciosa)
		slog.Warn("falha ao criar card no Pipefy", slog.String("email", salvo.Email), slog.Any("error", err))
	} else {
		slog.Info("card criado no Pipefy", slog.String("card_id", cardID), slog.String("email", salvo.Email))
	}

	// 4. Publicação de evento assíncrono no RabbitMQ (event-driven)
	event := output.ClienteCreatedEvent{
		ClienteID:       salvo.ID,
		Nome:            salvo.Nome,
		Email:           salvo.Email,
		TipoSolicitacao: salvo.TipoSolicitacao,
		ValorPatrimonio: salvo.ValorPatrimonio,
		Status:          salvo.Status,
	}
	if err := s.eventPublisher.PublicarClienteCriado(ctx, event); err != nil {
		slog.Warn("falha ao publicar evento no RabbitMQ", slog.String("email", salvo.Email), slog.Any("error", err))
	}

	return salvo, nil
}

// ProcessarWebhook aplica idempotência pelo event_id, calcula a prioridade com base
// no valor_patrimonio, aciona mutation updateCardField no Pipefy e atualiza o banco.
func (s *ClienteService) ProcessarWebhook(ctx context.Context, event *domain.WebhookEvent) error {
	// 1. Idempotência — bloqueia reprocessamento do mesmo event_id
	jaProcessado, err := s.webhookRepo.EventoJaProcessado(ctx, event.EventID)
	if err != nil {
		return err
	}
	if jaProcessado {
		return domain.ErrEventoDuplicado
	}

	// 2. Busca cliente pelo e-mail
	cliente, err := s.clienteRepo.BuscarPorEmail(ctx, event.ClienteEmail)
	if err != nil {
		return err
	}

	// 3. Regra de negócio: calcula prioridade com base no patrimônio
	prioridade := cliente.CalcularPrioridade()
	cliente.Prioridade = prioridade
	cliente.Status = domain.StatusProcessado

	// 4. Mapeamento Pipefy — mutation updateCardField (simulado)
	if err := s.pipefy.AtualizarCard(ctx, event.CardID, cliente.Status, prioridade); err != nil {
		slog.Warn("falha ao atualizar card no Pipefy", slog.String("card_id", event.CardID), slog.Any("error", err))
	} else {
		slog.Info("card atualizado no Pipefy", slog.String("card_id", event.CardID), slog.String("prioridade", string(prioridade)))
	}

	// 5. Atualiza banco local
	if err := s.clienteRepo.Atualizar(ctx, cliente); err != nil {
		return err
	}

	// 6. Registra evento para garantir idempotência futura
	return s.webhookRepo.RegistrarEvento(ctx, event)
}
