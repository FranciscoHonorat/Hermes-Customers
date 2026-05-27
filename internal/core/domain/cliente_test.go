package domain

import "testing"

func TestNewCliente_CamposObrigatorios(t *testing.T) {
	_, err := NewCliente("", "joao@example.com", "Atualização cadastral", 100000)
	if err != ErrCampoObrigatorio {
		t.Errorf("esperava ErrCampoObrigatorio, obteve: %v", err)
	}
}

func TestNewCliente_EmailInvalido(t *testing.T) {
	_, err := NewCliente("João", "nao-e-um-email", "Atualização cadastral", 100000)
	if err != ErrEmailInvalido {
		t.Errorf("esperava ErrEmailInvalido, obteve: %v", err)
	}
}

func TestNewCliente_Valido(t *testing.T) {
	c, err := NewCliente("João Silva", "joao@example.com", "Atualização cadastral", 250000)
	if err != nil {
		t.Fatalf("não esperava erro, obteve: %v", err)
	}
	if c.Status != StatusAguardandoAnalise {
		t.Errorf("status inicial esperado '%s', obteve '%s'", StatusAguardandoAnalise, c.Status)
	}
}

func TestCalcularPrioridade_Alta(t *testing.T) {
	c := &Cliente{ValorPatrimonio: 200_000}
	if c.CalcularPrioridade() != PrioridadeAlta {
		t.Error("patrimônio >= 200k deve resultar em prioridade_alta")
	}
}

func TestCalcularPrioridade_Normal(t *testing.T) {
	c := &Cliente{ValorPatrimonio: 199_999}
	if c.CalcularPrioridade() != PrioridadeNormal {
		t.Error("patrimônio < 200k deve resultar em prioridade_normal")
	}
}
