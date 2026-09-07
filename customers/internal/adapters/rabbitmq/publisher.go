// Package rabbitmq implementa a publicação de eventos via RabbitMQ.
// Segue o mesmo padrão do projeto movies (internal/adapters/rabbitmq/publisher.go).
package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/FranciscoHonorat/mundo-invest/customers/internal/core/ports/output"
	"shared"
)

const (
	ExchangeName = ""
	QueueName    = "clientes_criados"

	initialReconnectBackoff = time.Second
	maxReconnectBackoff     = 30 * time.Second
)

// Publisher encapsula a conexão e o canal AMQP para publicação de mensagens.
// Reconecta automaticamente (com backoff exponencial) se a conexão cair após
// o startup, para evitar perda silenciosa de eventos.
type Publisher struct {
	uri string

	mu      sync.RWMutex
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   amqp.Queue

	closeOnce sync.Once
	closeCh   chan struct{}
}

// NewPublisher cria um Publisher conectado ao RabbitMQ, declara a fila e
// inicia o monitor de reconexão em background.
func NewPublisher(uri string) (*Publisher, error) {
	p := &Publisher{uri: uri, closeCh: make(chan struct{})}
	if err := p.connect(); err != nil {
		return nil, err
	}
	go p.watchConnection()
	return p, nil
}

func (p *Publisher) connect() error {
	conn, err := amqp.Dial(p.uri)
	if err != nil {
		return fmt.Errorf("rabbitmq: conectar: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("rabbitmq: abrir canal: %w", err)
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
		return fmt.Errorf("rabbitmq: declarar fila: %w", err)
	}

	p.mu.Lock()
	p.conn = conn
	p.channel = ch
	p.queue = q
	p.mu.Unlock()

	slog.Info("RabbitMQ conectado", slog.String("fila", q.Name))
	return nil
}

// watchConnection observa o fechamento da conexão atual e dispara reconexão
// com backoff exponencial até a conexão voltar ou Close() ser chamado.
func (p *Publisher) watchConnection() {
	for {
		p.mu.RLock()
		conn := p.conn
		p.mu.RUnlock()

		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))

		select {
		case <-p.closeCh:
			return
		case err, ok := <-notifyClose:
			if !ok {
				return
			}
			slog.Warn("conexão RabbitMQ perdida, iniciando reconexão", slog.Any("error", err))
			if !p.redialWithBackoff() {
				return // Close() foi chamado durante a tentativa
			}
		}
	}
}

func (p *Publisher) redialWithBackoff() bool {
	backoff := initialReconnectBackoff
	for {
		select {
		case <-p.closeCh:
			return false
		default:
		}

		if err := p.connect(); err != nil {
			slog.Warn("falha ao reconectar RabbitMQ, tentando novamente",
				slog.Any("error", err), slog.Duration("backoff", backoff))
			select {
			case <-time.After(backoff):
			case <-p.closeCh:
				return false
			}
			if backoff < maxReconnectBackoff {
				backoff *= 2
			}
			continue
		}

		slog.Info("RabbitMQ reconectado com sucesso")
		return true
	}
}

// PublicarClienteCriado serializa e publica o evento na fila clientes_criados.
func (p *Publisher) PublicarClienteCriado(ctx context.Context, event output.ClienteCreatedEvent) error {
	message := shared.ClienteCreatedMessage{
		ClienteID:       event.ClienteID,
		Nome:            event.Nome,
		Email:           event.Email,
		TipoSolicitacao: event.TipoSolicitacao,
		ValorPatrimonio: event.ValorPatrimonio,
		Status:          string(event.Status),
	}

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("rabbitmq: serializar evento: %w", err)
	}

	p.mu.RLock()
	channel := p.channel
	queueName := p.queue.Name
	p.mu.RUnlock()

	err = channel.PublishWithContext(ctx,
		ExchangeName, // exchange (default)
		queueName,    // routing key = nome da fila
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // mensagem sobrevive a restart do broker
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("rabbitmq: publicar mensagem: %w", err)
	}

	slog.Info("evento publicado", slog.String("fila", queueName), slog.String("email", event.Email))
	return nil
}

// Close encerra graciosamente o monitor de reconexão, o canal e a conexão.
func (p *Publisher) Close() {
	p.closeOnce.Do(func() { close(p.closeCh) })

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// Garante que *Publisher implementa a interface em tempo de compilação.
var _ output.EventPublisher = (*Publisher)(nil)
