package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
)

func TestCardUpdated_PayloadValido_Retorna200(t *testing.T) {
	svc := &mockClienteService{
		ProcessarWebhookFn: func(ctx context.Context, e *domain.WebhookEvent) error {
			return nil
		},
	}
	h := NewWebhookHandler(svc)

	rec := doRequest(h.CardUpdated, http.MethodPost, "/webhooks/pipefy/card-updated", map[string]any{
		"event_id":      "evt_123",
		"card_id":       "card_456",
		"cliente_email": "joao@example.com",
		"timestamp":     "2026-05-18T12:00:00Z",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestCardUpdated_TimestampInvalido_Retorna400(t *testing.T) {
	svc := &mockClienteService{}
	h := NewWebhookHandler(svc)

	rec := doRequest(h.CardUpdated, http.MethodPost, "/webhooks/pipefy/card-updated", map[string]any{
		"event_id":      "evt_123",
		"card_id":       "card_456",
		"cliente_email": "joao@example.com",
		"timestamp":     "18/05/2026",
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestCardUpdated_EventoDuplicado_Retorna409(t *testing.T) {
	svc := &mockClienteService{
		ProcessarWebhookFn: func(ctx context.Context, e *domain.WebhookEvent) error {
			return domain.ErrEventoDuplicado
		},
	}
	h := NewWebhookHandler(svc)

	rec := doRequest(h.CardUpdated, http.MethodPost, "/webhooks/pipefy/card-updated", map[string]any{
		"event_id":      "evt_123",
		"card_id":       "card_456",
		"cliente_email": "joao@example.com",
		"timestamp":     "2026-05-18T12:00:00Z",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("esperava 409, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestCardUpdated_ClienteNaoEncontrado_Retorna404(t *testing.T) {
	svc := &mockClienteService{
		ProcessarWebhookFn: func(ctx context.Context, e *domain.WebhookEvent) error {
			return domain.ErrClienteNaoEncontrado
		},
	}
	h := NewWebhookHandler(svc)

	rec := doRequest(h.CardUpdated, http.MethodPost, "/webhooks/pipefy/card-updated", map[string]any{
		"event_id":      "evt_123",
		"card_id":       "card_456",
		"cliente_email": "joao@example.com",
		"timestamp":     "2026-05-18T12:00:00Z",
	})

	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperava 404, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestCardUpdated_ErroInesperado_Retorna500(t *testing.T) {
	svc := &mockClienteService{
		ProcessarWebhookFn: func(ctx context.Context, e *domain.WebhookEvent) error {
			return errUnexpected
		},
	}
	h := NewWebhookHandler(svc)

	rec := doRequest(h.CardUpdated, http.MethodPost, "/webhooks/pipefy/card-updated", map[string]any{
		"event_id":      "evt_123",
		"card_id":       "card_456",
		"cliente_email": "joao@example.com",
		"timestamp":     "2026-05-18T12:00:00Z",
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperava 500, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}
