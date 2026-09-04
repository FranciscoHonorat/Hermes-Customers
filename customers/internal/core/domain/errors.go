package domain

import "errors"

var (
	ErrCampoObrigatorio   = errors.New("campo obrigatório ausente")
	ErrEmailInvalido      = errors.New("e-mail inválido")
	ErrClienteNaoEncontrado = errors.New("cliente não encontrado")
	ErrEventoDuplicado    = errors.New("evento já processado (event_id duplicado)")
)
