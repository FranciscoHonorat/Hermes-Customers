# Mundo Invest — Backend API (Pipefy Integration)

API REST em Go para gerenciamento de clientes com integração simulada ao Pipefy via GraphQL,
publicação de eventos assíncronos no RabbitMQ e arquitetura hexagonal (ports & adapters).

---

## Estrutura de Pastas

```
mundo-invest/
├── cmd/
│   └── main.go                              # Entry point — composição das dependências
├── internal/
│   ├── core/                                # Camada interna: zero dependência de infraestrutura
│   │   ├── domain/
│   │   │   ├── cliente.go                   # Entidade Cliente + validação + cálculo de prioridade
│   │   │   ├── webhook_event.go             # Entidade WebhookEvent
│   │   │   ├── errors.go                    # Erros de domínio tipados
│   │   │   └── cliente_test.go              # Testes unitários de domínio
│   │   ├── ports/
│   │   │   ├── input/
│   │   │   │   └── cliente_service.go       # Interface de entrada (service)
│   │   │   └── output/
│   │   │       ├── cliente_repository.go    # Interfaces de persistência
│   │   │       ├── pipefy_client.go         # Interface do Pipefy GraphQL client
│   │   │       └── event_publisher.go       # Interface de publicação de eventos (RabbitMQ)
│   │   └── service/
│   │       ├── cliente_service.go           # Regras de negócio
│   │       └── cliente_service_test.go      # Testes unitários (mocks) — 4 casos
│   └── adapters/                            # Camada externa: implementações concretas
│       ├── sqlite/
│       │   ├── cliente_repository.go        # SQLite: clientes + webhook_events
│       │   └── cliente_repository_test.go   # Testes de INTEGRAÇÃO (SQLite :memory:)
│       ├── pipefy/
│       │   └── client.go                    # GraphQL mutations createCard / updateCardField
│       ├── rabbitmq/
│       │   ├── publisher.go                 # Publisher AMQP real
│       │   └── noop_publisher.go            # Noop para ambiente sem RabbitMQ
│       └── http/handlers/
│           ├── cliente_handler.go           # POST /clientes
│           ├── webhook_handler.go           # POST /webhooks/pipefy/card-updated
│           └── health_handler.go            # GET /health (probes k8s)
├── k8s/
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── api-deployment.yaml
│   ├── api-service.yaml
│   ├── sqlite-pvc.yaml
│   ├── rabbitmq-deployment.yaml
│   └── rabbitmq-service.yaml
├── go.mod
├── Dockerfile
├── docker-compose.yml
└── .env.example
```

**Fluxo de dependências (hexagonal):**
```
Handler → Service (port input) → [Repository port output | Pipefy port output | EventPublisher port output]
                                          ↓                        ↓                        ↓
                                    SQLite adapter          Pipefy adapter          RabbitMQ adapter
```
As camadas internas (domain, service, ports) nunca importam adapters — as interfaces são
injetadas via construtor (dependency injection) no `cmd/main.go`.

---

## Arquitetura Event-Driven

Ao criar um cliente via `POST /clientes`, além de persistir no SQLite e criar o card no Pipefy,
o service publica um evento `ClienteCreatedEvent` na fila `clientes_criados` do RabbitMQ.

```
POST /clientes
      │
      ▼
ClienteService.CriarCliente()
      ├── clienteRepo.Salvar()          → SQLite
      ├── pipefyClient.CriarCard()      → Pipefy GraphQL (mutation createCard)
      └── eventPublisher.Publicar()     → RabbitMQ (fila: clientes_criados)
```

Payload publicado na fila:
```json
{
  "cliente_id": 1,
  "nome": "João Silva",
  "email": "joao.silva@example.com",
  "tipo_solicitacao": "Atualização cadastral",
  "valor_patrimonio": 250000,
  "status": "Aguardando Análise"
}
```

Sem `RABBITMQ_URI` configurado, o `NoopPublisher` entra em ação e loga o evento
sem causar falha — degradação graciosa.

---

## Execução Local

### Pré-requisitos
- Go 1.22+
- Docker (opcional, para RabbitMQ)

### Modo simples (sem RabbitMQ)

```bash
cd mundo-invest
go mod tidy
go run ./cmd/main.go
```

### Com RabbitMQ (Docker Compose)

```bash
docker-compose up --build
```

RabbitMQ Management UI disponível em `http://localhost:15672` (guest/guest).

### Variáveis de Ambiente

| Variável            | Padrão              | Descrição                              |
|---------------------|---------------------|----------------------------------------|
| `PORT`              | `8080`              | Porta HTTP                             |
| `DATABASE_DSN`      | `mundo_invest.db`   | Caminho do arquivo SQLite              |
| `RABBITMQ_URI`      | _(vazio)_           | URI AMQP (ex: `amqp://guest:guest@localhost:5672/`) |
| `PIPEFY_API_TOKEN`  | _(vazio)_           | Token Bearer da API Pipefy             |
| `PIPEFY_PIPE_ID`    | _(vazio)_           | ID do Pipe onde os cards serão criados |

---

## Rodando os Testes

```bash
# Todos os testes (unitários + integração)
go test ./... -v

# Apenas testes unitários do service (mocks)
go test ./internal/core/service/... -v

# Apenas testes de integração (SQLite real :memory:)
go test ./internal/adapters/sqlite/... -v

# Apenas testes de domínio
go test ./internal/core/domain/... -v
```

### Testes unitários (`service/cliente_service_test.go`)

| Teste | Cobertura |
|-------|-----------|
| `TestCriarCliente_PayloadValido_SalvaNoBanco` | Criação com payload válido e salvamento no banco |
| `TestCriarCliente_EmailInvalido_RetornaErro` | Validação de e-mail inválido |
| `TestProcessarWebhook_PatrimonioAlto_PrioridadeAlta` | Patrimônio ≥ 200k → prioridade_alta |
| `TestProcessarWebhook_PatrimonioNormal_PrioridadeNormal` | Patrimônio < 200k → prioridade_normal |
| `TestProcessarWebhook_EventIDDuplicado_Bloqueado` | Idempotência: bloqueia event_id duplicado |
| `TestCriarCliente_PublicaEventoRabbitMQ` | Evento publicado no RabbitMQ após criar cliente |

### Testes de integração (`sqlite/cliente_repository_test.go`)

| Teste | Cobertura |
|-------|-----------|
| `TestIntegration_Salvar_BuscarPorEmail` | Salva e recupera cliente real do SQLite |
| `TestIntegration_BuscarPorEmail_NaoEncontrado` | Retorna erro para e-mail inexistente |
| `TestIntegration_Atualizar_StatusEPrioridade` | Persiste atualização de status e prioridade |
| `TestIntegration_EmailUnico_NaoPermiteDuplicata` | Constraint UNIQUE no e-mail |
| `TestIntegration_EventoJaProcessado_False` | Evento novo retorna false |
| `TestIntegration_RegistrarEvento_Idempotencia` | Bloqueia event_id duplicado via constraint |
| `TestIntegration_FluxoCompleto_CriarEProcessarWebhook` | Fluxo end-to-end completo via repositório |

---

## Exemplos de Requisição (curl)

### GET /health

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

### POST /clientes

```bash
curl -X POST http://localhost:8080/clientes \
  -H "Content-Type: application/json" \
  -d '{
    "cliente_nome": "João Silva",
    "cliente_email": "joao.silva@example.com",
    "tipo_solicitacao": "Atualização cadastral",
    "valor_patrimonio": 250000
  }'
```

**Resposta 201:**
```json
{
  "id": 1,
  "cliente_nome": "João Silva",
  "cliente_email": "joao.silva@example.com",
  "tipo_solicitacao": "Atualização cadastral",
  "valor_patrimonio": 250000,
  "status": "Aguardando Análise"
}
```

**Resposta 400 (e-mail inválido):**
```json
{ "error": "e-mail inválido" }
```

---

### POST /webhooks/pipefy/card-updated

```bash
curl -X POST http://localhost:8080/webhooks/pipefy/card-updated \
  -H "Content-Type: application/json" \
  -d '{
    "event_id": "evt_123",
    "card_id": "card_456",
    "cliente_email": "joao.silva@example.com",
    "timestamp": "2026-05-18T12:00:00Z"
  }'
```

**Resposta 200:**
```json
{ "message": "webhook processado com sucesso" }
```

**Resposta 409 (event_id duplicado — idempotência):**
```json
{ "error": "evento já processado (event_id duplicado)" }
```

---

## Mutations GraphQL do Pipefy

O código completo está em `internal/adapters/pipefy/client.go`, com links para a documentação oficial.  
Spec: https://developers.pipefy.com/reference/mutations-cards

### createCard

```graphql
mutation createCard($input: CreateCardInput!) {
  createCard(input: $input) {
    card {
      id
      title
      current_phase { name }
    }
  }
}
```

Variáveis:
```json
{
  "input": {
    "pipe_id": "<PIPEFY_PIPE_ID>",
    "title": "João Silva",
    "fields_attributes": [
      { "field_id": "email",       "field_value": "joao.silva@example.com" },
      { "field_id": "patrimonio",  "field_value": "250000.00" },
      { "field_id": "solicitacao", "field_value": "Atualização cadastral" }
    ]
  }
}
```

### updateCardField

```graphql
mutation updateCardField($input: UpdateCardFieldInput!) {
  updateCardField(input: $input) {
    card { id title }
    success
  }
}
```

Duas chamadas sequenciais (um campo por chamada):
```json
{ "input": { "card_id": "card_456", "field_id": "status",     "new_value": "Processado"      } }
{ "input": { "card_id": "card_456", "field_id": "prioridade", "new_value": "prioridade_alta" } }
```

---

## Deploy Kubernetes

```bash
# Aplicar todos os manifests
kubectl apply -f k8s/

# Verificar pods
kubectl get pods

# Acompanhar logs da API
kubectl logs -f deployment/mundo-invest-api

# Acessar a API (port-forward)
kubectl port-forward svc/mundo-invest-api 8080:80
```

Manifests disponíveis em `k8s/`:

| Arquivo | Descrição |
|---------|-----------|
| `configmap.yaml` | Variáveis de ambiente não-secretas |
| `secret.yaml` | Token Pipefy e Pipe ID |
| `sqlite-pvc.yaml` | PersistentVolumeClaim de 1Gi para o SQLite |
| `api-deployment.yaml` | Deployment da API (2 réplicas, probes, resources) |
| `api-service.yaml` | ClusterIP service na porta 80 |
| `rabbitmq-deployment.yaml` | RabbitMQ com management UI |
| `rabbitmq-service.yaml` | ClusterIP para AMQP (5672) e management (15672) |

---

## Visão de Produção (AWS)

**API Gateway + Lambda:** cada endpoint (`POST /clientes` e `POST /webhooks/pipefy/card-updated`)
seria uma função Lambda independente atrás do API Gateway. O Lambda escala automaticamente
por invocação, sem gerenciamento de servidores.

**Event-driven:** ao criar um cliente, em vez do RabbitMQ local, o Lambda publicaria uma mensagem
no **Amazon SQS** (ou SNS + SQS fan-out). Uma segunda Lambda consumiria a fila para processar
notificações, relatórios e outras integrações downstream — totalmente desacoplado.

**Banco de dados:** o SQLite seria substituído por **Amazon RDS (PostgreSQL)** para workloads
transacionais com múltiplas instâncias Lambda, ou por **DynamoDB** se a prioridade for escala
horizontal extrema (email como partition key, event_id como chave única para idempotência via
`ConditionExpression`).

**Idempotência em escala:** o controle de `event_id` duplicado ficaria numa tabela DynamoDB
com TTL de 7 dias, garantindo atomicidade mesmo com Lambdas concorrentes via
`ConditionExpression: "attribute_not_exists(event_id)"`.

**Segredos:** `PIPEFY_API_TOKEN` e credenciais do banco ficariam no **AWS Secrets Manager**,
injetados nas Lambdas em tempo de execução.

**Observabilidade:** logs estruturados (`slog`) vão automaticamente ao **CloudWatch Logs**;
métricas de latência e erros via **CloudWatch Metrics**; alarmes no **SNS** para o time de ops.
