package handlers

import (
	"net/http"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/ports/input"
	"github.com/gin-gonic/gin"
)

// ClienteHandler expõe o endpoint POST /clientes.
type ClienteHandler struct {
	svc input.ClienteService
}

func NewClienteHandler(svc input.ClienteService) *ClienteHandler {
	return &ClienteHandler{svc: svc}
}

// CreateClienteRequest representa o payload de entrada do endpoint.
type CreateClienteRequest struct {
	ClienteNome     string  `json:"cliente_nome"      binding:"required"`
	ClienteEmail    string  `json:"cliente_email"     binding:"required"`
	TipoSolicitacao string  `json:"tipo_solicitacao"  binding:"required"`
	ValorPatrimonio float64 `json:"valor_patrimonio"  binding:"required"`
}

// CriarCliente godoc
// @Summary      Criar novo cliente
// @Description  Valida, persiste o cliente com status "Aguardando Análise" e cria card no Pipefy.
// @Tags         Clientes
// @Accept       json
// @Produce      json
// @Param        body  body      CreateClienteRequest  true  "Dados do cliente"
// @Success      201   {object}  map[string]any
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /clientes [post]
func (h *ClienteHandler) CriarCliente(c *gin.Context) {
	var req CreateClienteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON inválido ou campos obrigatórios ausentes"})
		return
	}

	cliente, err := domain.NewCliente(req.ClienteNome, req.ClienteEmail, req.TipoSolicitacao, req.ValorPatrimonio)
	if err != nil {
		statusCode := http.StatusBadRequest
		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	criado, err := h.svc.CriarCliente(c.Request.Context(), cliente)
	if err != nil {
		switch err {
		case domain.ErrEmailInvalido, domain.ErrCampoObrigatorio:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":               criado.ID,
		"cliente_nome":     criado.Nome,
		"cliente_email":    criado.Email,
		"tipo_solicitacao": criado.TipoSolicitacao,
		"valor_patrimonio": criado.ValorPatrimonio,
		"status":           criado.Status,
	})
}
