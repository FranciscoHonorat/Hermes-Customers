package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
	"github.com/gin-gonic/gin"
)

var errUnexpected = errors.New("erro inesperado de teste")

func init() {
	gin.SetMode(gin.TestMode)
}

type mockClienteService struct {
	CriarClienteFn     func(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error)
	ProcessarWebhookFn func(ctx context.Context, e *domain.WebhookEvent) error
}

func (m *mockClienteService) CriarCliente(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
	return m.CriarClienteFn(ctx, c)
}

func (m *mockClienteService) ProcessarWebhook(ctx context.Context, e *domain.WebhookEvent) error {
	return m.ProcessarWebhookFn(ctx, e)
}

func doRequest(handler gin.HandlerFunc, method, path string, body any) *httptest.ResponseRecorder {
	r := gin.New()
	r.Handle(method, path, handler)

	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestCriarCliente_PayloadValido_Retorna201(t *testing.T) {
	svc := &mockClienteService{
		CriarClienteFn: func(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
			c.ID = 1
			return c, nil
		},
	}
	h := NewClienteHandler(svc)

	rec := doRequest(h.CriarCliente, http.MethodPost, "/clientes", map[string]any{
		"cliente_nome":     "João Silva",
		"cliente_email":    "joao@example.com",
		"tipo_solicitacao": "Abertura de conta",
		"valor_patrimonio": 250000,
	})

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperava 201, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestCriarCliente_JSONInvalido_Retorna400(t *testing.T) {
	svc := &mockClienteService{}
	h := NewClienteHandler(svc)

	r := gin.New()
	r.POST("/clientes", h.CriarCliente)
	req := httptest.NewRequest(http.MethodPost, "/clientes", bytes.NewBufferString("{invalido"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400, obteve %d", rec.Code)
	}
}

func TestCriarCliente_EmailInvalido_Retorna400(t *testing.T) {
	svc := &mockClienteService{}
	h := NewClienteHandler(svc)

	rec := doRequest(h.CriarCliente, http.MethodPost, "/clientes", map[string]any{
		"cliente_nome":     "João",
		"cliente_email":    "nao-e-email",
		"tipo_solicitacao": "Abertura de conta",
		"valor_patrimonio": 1000,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestCriarCliente_EmailDuplicado_Retorna409(t *testing.T) {
	svc := &mockClienteService{
		CriarClienteFn: func(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
			return nil, domain.ErrEmailDuplicado
		},
	}
	h := NewClienteHandler(svc)

	rec := doRequest(h.CriarCliente, http.MethodPost, "/clientes", map[string]any{
		"cliente_nome":     "João",
		"cliente_email":    "joao@example.com",
		"tipo_solicitacao": "Abertura de conta",
		"valor_patrimonio": 1000,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("esperava 409, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestCriarCliente_BancoIndisponivel_Retorna503(t *testing.T) {
	svc := &mockClienteService{
		CriarClienteFn: func(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
			return nil, domain.ErrBancoIndisponivel
		},
	}
	h := NewClienteHandler(svc)

	rec := doRequest(h.CriarCliente, http.MethodPost, "/clientes", map[string]any{
		"cliente_nome":     "João",
		"cliente_email":    "joao@example.com",
		"tipo_solicitacao": "Abertura de conta",
		"valor_patrimonio": 1000,
	})

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperava 503, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}

func TestCriarCliente_ErroInesperado_Retorna500(t *testing.T) {
	svc := &mockClienteService{
		CriarClienteFn: func(ctx context.Context, c *domain.Cliente) (*domain.Cliente, error) {
			return nil, errUnexpected
		},
	}
	h := NewClienteHandler(svc)

	rec := doRequest(h.CriarCliente, http.MethodPost, "/clientes", map[string]any{
		"cliente_nome":     "João",
		"cliente_email":    "joao@example.com",
		"tipo_solicitacao": "Abertura de conta",
		"valor_patrimonio": 1000,
	})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperava 500, obteve %d — corpo: %s", rec.Code, rec.Body.String())
	}
}
