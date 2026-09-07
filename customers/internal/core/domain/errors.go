package domain

import "errors"

var (
	ErrCampoObrigatorio     = errors.New("campo obrigatório ausente")
	ErrEmailInvalido        = errors.New("e-mail inválido")
	ErrEmailDuplicado       = errors.New("e-mail já cadastrado")
	ErrPatrimonioInvalido   = errors.New("valor_patrimonio não pode ser negativo")
	ErrClienteNaoEncontrado = errors.New("cliente não encontrado")
	ErrEventoDuplicado      = errors.New("evento já processado (event_id duplicado)")
)
