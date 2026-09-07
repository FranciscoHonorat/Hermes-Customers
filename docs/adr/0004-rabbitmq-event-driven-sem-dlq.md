# ADR-0004: Publicação de eventos via RabbitMQ com degradação graciosa

## Status
Aceita (retroativa)

## Contexto
Ao criar um cliente, o sistema precisa notificar consumidores downstream
(notificações, relatórios) sem acoplar `CriarCliente` a esses consumidores, e
sem que a ausência de RabbitMQ quebre o fluxo principal (persistência +
Pipefy) em ambientes de desenvolvimento/teste.

## Decisão
Publicar `ClienteCreatedEvent` na fila `clientes_criados` (exchange padrão,
`DeliveryMode: Persistent`) via `output.EventPublisher`. Quando
`RABBITMQ_URI` está vazio, ou a conexão inicial falha, o serviço usa
`NoopPublisher`, que apenas loga o evento e retorna sucesso.

## Consequências
- **Positivo:** o serviço nunca fica fora do ar por causa do RabbitMQ; falha
  de publicação é logada como `Warn`, não propagada como erro de
  `CriarCliente`.
- **Negativo (achado da auditoria de 2026-09-07):** essa mesma degradação
  graciosa se aplica também a falhas *depois* do boot — se a conexão AMQP
  cair após `NewPublisher` ter sucesso, `PublishWithContext` passa a falhar e
  o evento é só logado como warning, sem retry, sem fila de dead-letter e sem
  reconexão automática. Para um fluxo que se declara "event-driven" isso é uma
  perda silenciosa de eventos. Precisa de uma decisão explícita: aceitar a
  perda (at-most-once, documentado) ou adicionar retry/DLQ/reconexão
  (at-least-once).

## Atualização (2026-09-07)
`Publisher` (`customers/internal/adapters/rabbitmq/publisher.go`) passou a
monitorar `conn.NotifyClose` e reconectar com backoff exponencial (1s a 30s)
em background quando a conexão cai após o boot — resolve a perda silenciosa
por queda de conexão. Segue sem DLQ: uma mensagem que falha na publicação em
si (não por conexão caída) continua só logada como warning. Fila de
dead-letter fica como possível ADR futura, não coberta aqui.
