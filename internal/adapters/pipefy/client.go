// Package pipefy implementa a integração com a API GraphQL do Pipefy.
//
// As mutations seguem rigorosamente a especificação oficial:
// https://developers.pipefy.com/reference/mutations-cards
//
// Em produção, basta fornecer PIPEFY_API_TOKEN e PIPEFY_PIPE_ID nas variáveis
// de ambiente. Neste ambiente de teste as chamadas são SIMULADAS (logged), pois
// não há token real configurado.
package pipefy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/FranciscoHonorat/mundo-invest/internal/core/domain"
)

const pipefyAPIURL = "https://api.pipefy.com/graphql"

// Client encapsula a comunicação HTTP com a API GraphQL do Pipefy.
type Client struct {
	httpClient *http.Client
	apiToken   string
	pipeID     string
	simulated  bool // true quando não há token configurado (ambiente de teste)
}

// NewClient cria um Client lendo as variáveis de ambiente.
// Se PIPEFY_API_TOKEN não estiver definido, opera em modo simulado (sem chamadas reais).
func NewClient() *Client {
	token := os.Getenv("PIPEFY_API_TOKEN")
	pipeID := os.Getenv("PIPEFY_PIPE_ID")
	simulated := token == ""

	if simulated {
		slog.Warn("PIPEFY_API_TOKEN não configurado — operando em modo simulado (sem chamadas reais ao Pipefy)")
	}

	return &Client{
		httpClient: &http.Client{},
		apiToken:   token,
		pipeID:     pipeID,
		simulated:  simulated,
	}
}

// ---------------------------------------------------------------------------
// Tipos internos para serialização GraphQL
// ---------------------------------------------------------------------------

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ---------------------------------------------------------------------------
// CriarCard — mutation createCard (spec oficial Pipefy)
// ---------------------------------------------------------------------------
//
// Mutation utilizada (documentação: https://developers.pipefy.com/reference/mutations-cards#createcard):
//
//	mutation createCard($input: CreateCardInput!) {
//	  createCard(input: $input) {
//	    card {
//	      id
//	      title
//	      current_phase { name }
//	    }
//	  }
//	}
//
// Variáveis enviadas:
//
//	{
//	  "input": {
//	    "pipe_id": "<PIPEFY_PIPE_ID>",
//	    "title": "<cliente_nome>",
//	    "fields_attributes": [
//	      { "field_id": "email",      "field_value": "<cliente_email>" },
//	      { "field_id": "patrimonio", "field_value": "<valor_patrimonio>" },
//	      { "field_id": "solicitacao","field_value": "<tipo_solicitacao>" }
//	    ]
//	  }
//	}

func (c *Client) CriarCard(ctx context.Context, cliente *domain.Cliente) (string, error) {
	const mutation = `
mutation createCard($input: CreateCardInput!) {
  createCard(input: $input) {
    card {
      id
      title
      current_phase {
        name
      }
    }
  }
}`

	variables := map[string]any{
		"input": map[string]any{
			"pipe_id": c.pipeID,
			"title":   cliente.Nome,
			"fields_attributes": []map[string]string{
				{"field_id": "email", "field_value": cliente.Email},
				{"field_id": "patrimonio", "field_value": strconv.FormatFloat(cliente.ValorPatrimonio, 'f', 2, 64)},
				{"field_id": "solicitacao", "field_value": cliente.TipoSolicitacao},
			},
		},
	}

	if c.simulated {
		payload, _ := json.MarshalIndent(graphQLRequest{Query: mutation, Variables: variables}, "", "  ")
		slog.Info("[PIPEFY SIMULADO] createCard", slog.String("payload", string(payload)))
		return "card_simulado_" + strconv.FormatInt(cliente.ID, 10), nil
	}

	resp, err := c.doRequest(ctx, mutation, variables)
	if err != nil {
		return "", err
	}

	// Extrai o id do card da resposta
	var result struct {
		CreateCard struct {
			Card struct {
				ID string `json:"id"`
			} `json:"card"`
		} `json:"createCard"`
	}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return "", fmt.Errorf("pipefy: decodificar resposta createCard: %w", err)
	}
	return result.CreateCard.Card.ID, nil
}

// ---------------------------------------------------------------------------
// AtualizarCard — mutation updateCardField (spec oficial Pipefy)
// ---------------------------------------------------------------------------
//
// Mutation utilizada (documentação: https://developers.pipefy.com/reference/mutations-cards#updatecardfield):
//
//	mutation updateCardField($input: UpdateCardFieldInput!) {
//	  updateCardField(input: $input) {
//	    card {
//	      id
//	      title
//	    }
//	    success
//	  }
//	}
//
// São enviadas duas chamadas sequenciais — uma para atualizar o status e outra
// para atualizar a prioridade — pois updateCardField atualiza um campo por vez.
//
// Variáveis (status):
//
//	{ "input": { "card_id": "<card_id>", "field_id": "status",     "new_value": "Processado"      } }
//
// Variáveis (prioridade):
//
//	{ "input": { "card_id": "<card_id>", "field_id": "prioridade", "new_value": "prioridade_alta" } }

func (c *Client) AtualizarCard(ctx context.Context, cardID string, status domain.StatusCliente, prioridade domain.Prioridade) error {
	const mutation = `
mutation updateCardField($input: UpdateCardFieldInput!) {
  updateCardField(input: $input) {
    card {
      id
      title
    }
    success
  }
}`

	campos := []struct {
		fieldID string
		value   string
	}{
		{"status", string(status)},
		{"prioridade", string(prioridade)},
	}

	for _, campo := range campos {
		variables := map[string]any{
			"input": map[string]any{
				"card_id":   cardID,
				"field_id":  campo.fieldID,
				"new_value": campo.value,
			},
		}

		if c.simulated {
			payload, _ := json.MarshalIndent(graphQLRequest{Query: mutation, Variables: variables}, "", "  ")
			slog.Info("[PIPEFY SIMULADO] updateCardField", slog.String("campo", campo.fieldID), slog.String("payload", string(payload)))
			continue
		}

		if _, err := c.doRequest(ctx, mutation, variables); err != nil {
			return fmt.Errorf("pipefy: atualizar campo '%s': %w", campo.fieldID, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (c *Client) doRequest(ctx context.Context, query string, variables map[string]any) (*graphQLResponse, error) {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("pipefy: serializar request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pipefyAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("pipefy: criar request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pipefy: enviar request: %w", err)
	}
	defer httpResp.Body.Close()

	var gqlResp graphQLResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("pipefy: decodificar response: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("pipefy: erro GraphQL: %s", gqlResp.Errors[0].Message)
	}
	return &gqlResp, nil
}
