// Package rabbitmq implementa a publicação de eventos via RabbitMQ.
// Segue o mesmo padrão do projeto movies (internal/adapters/rabbitmq/publisher.go).
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/FranciscoHonorat/mundo-invest/internal/core/ports/output"
)

const (
	ExchangeName = ""
	QueueName    = "clientes_criados"
)

// Publisher encapsula a conexão e o canal AMQP para publicação de mensagens.
type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   amqp.Queue
}

// NewPublisher cria um Publisher conectado ao RabbitMQ e declara a fila.
func NewPublisher(uri string) (*Publisher, error) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: conectar: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: abrir canal: %w", err)
	}

	q, err := ch.QueueDeclare(
		QueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: declarar fila: %w", err)
	}

	slog.Info("RabbitMQ conectado", slog.String("fila", q.Name))
	return &Publisher{conn: conn, channel: ch, queue: q}, nil
}

// PublicarClienteCriado serializa e publica o evento na fila clientes_criados.
func (p *Publisher) PublicarClienteCriado(ctx context.Context, event output.ClienteCreatedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("rabbitmq: serializar evento: %w", err)
	}

	err = p.channel.PublishWithContext(ctx,
		ExchangeName,  // exchange (default)
		p.queue.Name,  // routing key = nome da fila
		false,         // mandatory
		false,         // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // mensagem sobrevive a restart do broker
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: publicar mensagem: %w", err)
	}

	slog.Info("evento publicado", slog.String("fila", p.queue.Name), slog.String("email", event.Email))
	return nil
}

// Close encerra graciosamente canal e conexão.
func (p *Publisher) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// Garante que *Publisher implementa a interface em tempo de compilação.
var _ output.EventPublisher = (*Publisher)(nil)
