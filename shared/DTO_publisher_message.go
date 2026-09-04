package shared

type ClienteCreatedMessage struct {
	ClienteID       int64   `json:"cliente_id"`
	Nome            string  `json:"nome"`
	Email           string  `json:"email"`
	TipoSolicitacao string  `json:"tipo_solicitacao"`
	ValorPatrimonio float64 `json:"valor_patrimonio"`
	Status          string  `json:"status"`
}
