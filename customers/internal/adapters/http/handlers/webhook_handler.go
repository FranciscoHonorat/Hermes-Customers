package handlers

import (
	"net/http"
	"time"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/domain"
	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/ports/input"
	"github.com/gin-gonic/gin"
)

// WebhookHandler expõe o endpoint POST /webhooks/pipefy/card-updated.
type WebhookHandler struct {
	svc input.ClienteService
}

func NewWebhookHandler(svc input.ClienteService) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

// CardUpdatedRequest representa o payload enviado pelo Pipefy via webhook.
type CardUpdatedRequest struct {
	EventID      string `json:"event_id"      binding:"required"`
	CardID       string `json:"card_id"       binding:"required"`
	ClienteEmail string `json:"cliente_email" binding:"required"`
	Timestamp    string `json:"timestamp"     binding:"required"`
}

// CardUpdated godoc
// @Summary      Processar webhook de atualização de card
// @Description  Aplica idempotência, calcula prioridade e atualiza status no banco e no Pipefy.
// @Tags         Webhooks
// @Accept       json
// @Produce      json
// @Param        body  body      CardUpdatedRequest  true  "Payload do webhook Pipefy"
// @Success      200   {object}  map[string]string
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /webhooks/pipefy/card-updated [post]
func (h *WebhookHandler) CardUpdated(c *gin.Context) {
	var req CardUpdatedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON inválido ou campos obrigatórios ausentes"})
		return
	}

	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "timestamp inválido, use RFC3339 (ex: 2006-01-02T15:04:05Z)"})
		return
	}

	event := &domain.WebhookEvent{
		EventID:      req.EventID,
		CardID:       req.CardID,
		ClienteEmail: req.ClienteEmail,
		Timestamp:    ts,
	}

	if err := h.svc.ProcessarWebhook(c.Request.Context(), event); err != nil {
		switch err {
		case domain.ErrEventoDuplicado:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case domain.ErrClienteNaoEncontrado:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "webhook processado com sucesso"})
}
