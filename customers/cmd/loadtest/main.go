// Comando loadtest publica N eventos sintéticos na fila "clientes_criados"
// para medir throughput de ingestão via RabbitMQ — Fase 2 do roteiro de
// verificação de performance (docs/testing/roteiro-verificacao-performance.md).
//
// Uso:
//
//	go run ./cmd/loadtest -n 10000 -uri amqp://guest:guest@localhost:5672/
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	uri := flag.String("uri", "amqp://guest:guest@localhost:5672/", "URI AMQP do RabbitMQ")
	total := flag.Int("n", 10_000, "número de eventos a publicar")
	queue := flag.String("queue", "clientes_criados", "fila de destino")
	flag.Parse()

	conn, err := amqp.Dial(*uri)
	if err != nil {
		log.Fatalf("conectar: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("abrir canal: %v", err)
	}
	defer ch.Close()

	if _, err := ch.QueueDeclare(*queue, true, false, false, false, nil); err != nil {
		log.Fatalf("declarar fila: %v", err)
	}

	ctx := context.Background()
	start := time.Now()

	for i := 0; i < *total; i++ {
		body, err := json.Marshal(map[string]any{
			"cliente_id":       i,
			"nome":             fmt.Sprintf("Loadtest %d", i),
			"email":            fmt.Sprintf("loadtest%d_%d@example.com", start.Unix(), i),
			"tipo_solicitacao": "Abertura de conta",
			"valor_patrimonio": float64(i%500_000) + 1,
			"status":           "Aguardando Análise",
		})
		if err != nil {
			log.Fatalf("serializar evento %d: %v", i, err)
		}

		err = ch.PublishWithContext(ctx, "", *queue, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
		if err != nil {
			log.Fatalf("publicar evento %d: %v", i, err)
		}
	}

	elapsed := time.Since(start)
	fmt.Printf("Publicados %d eventos em %s (%.1f eventos/s)\n",
		*total, elapsed, float64(*total)/elapsed.Seconds())
}
