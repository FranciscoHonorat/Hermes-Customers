package domain

import (
	"regexp"
	"strings"
)

// Prioridade representa o nível de prioridade de um cliente.
type Prioridade string

const (
	PrioridadeAlta   Prioridade = "prioridade_alta"
	PrioridadeNormal Prioridade = "prioridade_normal"
)

// StatusCliente representa o ciclo de vida do cliente no sistema.
type StatusCliente string

const (
	StatusAguardandoAnalise StatusCliente = "Aguardando Análise"
	StatusProcessado        StatusCliente = "Processado"
)

// Cliente é a entidade central do domínio.
type Cliente struct {
	ID              int64
	Nome            string
	Email           string
	TipoSolicitacao string
	ValorPatrimonio float64
	Status          StatusCliente
	Prioridade      Prioridade
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Validate verifica se os campos obrigatórios estão presentes e se o e-mail é válido.
func (c *Cliente) Validate() error {
	if strings.TrimSpace(c.Nome) == "" {
		return ErrCampoObrigatorio
	}
	if strings.TrimSpace(c.Email) == "" {
		return ErrCampoObrigatorio
	}
	if !emailRegex.MatchString(c.Email) {
		return ErrEmailInvalido
	}
	if strings.TrimSpace(c.TipoSolicitacao) == "" {
		return ErrCampoObrigatorio
	}
	return nil
}

// CalcularPrioridade aplica a regra de negócio de prioridade com base no patrimônio.
func (c *Cliente) CalcularPrioridade() Prioridade {
	if c.ValorPatrimonio >= 200_000 {
		return PrioridadeAlta
	}
	return PrioridadeNormal
}

// NewCliente cria um novo Cliente já validado com status inicial.
func NewCliente(nome, email, tipoSolicitacao string, valorPatrimonio float64) (*Cliente, error) {
	c := &Cliente{
		Nome:            nome,
		Email:           email,
		TipoSolicitacao: tipoSolicitacao,
		ValorPatrimonio: valorPatrimonio,
		Status:          StatusAguardandoAnalise,
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}
