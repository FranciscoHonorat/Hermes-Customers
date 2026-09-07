# ADR-0001: Arquitetura hexagonal (ports & adapters) no serviço `customers`

## Status
Aceita (retroativa — já implementada em `customers/internal/{core,adapters}`)

## Contexto
O serviço `customers` integra três dependências externas com ciclos de vida e
maturidade diferentes: persistência (SQLite hoje, Postgres/DynamoDB no futuro),
o GraphQL do Pipefy, e o RabbitMQ. A regra de negócio (cálculo de prioridade,
validação de cliente, idempotência de webhook) precisa ser testável sem subir
nenhuma dessas dependências, e cada adapter precisa poder ser trocado
isoladamente conforme o `README.md` já prevê na seção "Visão de Produção (AWS)".

## Decisão
Organizar o serviço em `core/domain`, `core/ports/{input,output}` e
`core/service` (sem nenhuma dependência de infraestrutura), com
implementações concretas isoladas em `internal/adapters/{sqlite,pipefy,rabbitmq,http}`.
A injeção de dependências é feita manualmente em `cmd/main.go`, sem container de DI.

## Consequências
- **Positivo:** `cliente_service_test.go` testa as regras de negócio com mocks,
  sem precisar de banco ou RabbitMQ; `cliente_repository_test.go` testa a
  camada de dados de forma isolada com SQLite `:memory:`.
- **Positivo:** trocar SQLite por Postgres, ou o `NoopPublisher` por um
  publisher real, não deveria exigir mudanças em `core/`.
- **Negativo:** camada de adapters HTTP (`cliente_handler.go`,
  `webhook_handler.go`) ficou sem nenhum teste — a auditoria de 2026-09-07
  identificou isso como pendência (ver relatório de auditoria). **Resolvido**
  em 2026-09-07: `cliente_handler_test.go` e `webhook_handler_test.go` cobrem
  os status codes de cada branch de erro (400/401/404/409/500), e os novos
  middlewares (`auth.go`, `webhook_signature.go`) têm testes próprios.
- **Negativo:** overhead de indireção para um serviço com apenas dois
  endpoints; aceitável dado o objetivo explícito de demonstrar o padrão.
