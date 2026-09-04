package domain

import "time"

// WebhookEvent representa um evento recebido do Pipefy via webhook.
type WebhookEvent struct {
	ID           int64
	EventID      string
	CardID       string
	ClienteEmail string
	Timestamp    time.Time
	Processado   bool
}
