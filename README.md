# Mundo Invest — Backend API (Pipefy Integration)

API REST em Go para gerenciamento de clientes com integração simulada ao Pipefy via GraphQL,
publicação de eventos assíncronos no RabbitMQ e arquitetura hexagonal (ports & adapters).

---

## Performance

Medido com `hey`/`vegeta` em 2026-09-07, ambiente local via `docker-compose`
(não é hardware de produção — números comparáveis entre execuções, não uma
garantia de capacidade). Detalhes e comandos exatos em
[`docs/testing/resultados-2026-09-07.md`](docs/testing/resultados-2026-09-07.md).

| Métrica | Valor | Comando |
|---|---|---|
| Throughput leitura (`GET /health`, customers direto) | 34.714 req/s | `hey -z 15s -c 50 $URL/health` |
| Throughput leitura (via api-gateway) | 33.837 req/s (~2,5% overhead do proxy) | `hey -z 15s -c 50 $URL/health` |
| P99 leitura | 5.1 ms | idem |
| Throughput escrita (`POST /clientes`, baixa concorrência) | 100% sucesso, p99 11.5 ms | `vegeta attack -rate=40/1s -duration=10s` |
| Concorrência de escrita (5 a 50 requisições simultâneas) | 100% sucesso (WAL + `busy_timeout` + pool de 1 conexão — antes do fix: >98% de erro 500) | `vegeta attack -workers=5..50` |
| Ingestão RabbitMQ (publish, sem consumidor) | 74.579 eventos/s | `go run ./customers/cmd/loadtest -n 10000` |
| Memória em repouso (customers) | 22.6 MiB | `docker stats` |

O teste de concorrência forçada achou um limite real (SQLite falhando sob
escrita simultânea), que foi corrigido e revalidado no mesmo dia — histórico
completo, com números de antes/depois e o trade-off de latência sob carga,
em [`docs/testing/resultados-2026-09-07.md`](docs/testing/resultados-2026-09-07.md).
Reproduza com
[`docs/testing/roteiro-verificacao-performance.md`](docs/testing/roteiro-verificacao-performance.md).

---

## Estrutura de Pastas

Monorepo com três módulos Go independentes, unidos por um [Go workspace](https://go.dev/ref/mod#workspaces)
(`go.work`) para desenvolvimento local — cada um com seu próprio `go.mod`:

```
Hermes-Customers/
├── customers/                                 # módulo github.com/.../mundo-invest/customers
│   ├── cmd/main.go                            # Entry point — composição das dependências
│   ├── internal/
│   │   ├── core/                              # zero dependência de infraestrutura
│   │   │   ├── domain/                        # Cliente, WebhookEvent, erros tipados
│   │   │   ├── ports/{input,output}/          # interfaces de entrada/saída
│   │   │   └── service/                       # regras de negócio
│   │   └── adapters/                          # implementações concretas
│   │       ├── sqlite/                        # clientes + webhook_events
│   │       ├── pipefy/                        # GraphQL createCard / updateCardField
│   │       ├── rabbitmq/                      # publisher (com reconexão) + noop
│   │       └── http/
│   │           ├── handlers/                  # POST /clientes, /webhooks/..., /health
│   │           └── middleware/                # auth, assinatura de webhook, request-id, métricas
│   ├── go.mod · go.sum · Dockerfile
├── api-gateway/                                # módulo api-gateway — ponto de entrada único
│   ├── cmd/main.go
│   ├── internal/{adapters/proxy,handlers,middleware}/  # proxy, CORS, rate limit, métricas
│   └── go.mod · go.sum · Dockerfile
├── shared/                                     # módulo shared — DTOs compartilhados (eventos)
├── k8s/                                        # manifests de Deployment/Service/Ingress/NetworkPolicy
├── docs/adr/                                   # Architecture Decision Records
├── docs/LEARNING.md                            # aprendizados técnicos registrados
├── .github/workflows/                          # CI (lint/test) e build & publish de imagens
├── go.work · go.work.sum                       # workspace Go (liga os 3 módulos localmente)
├── docker-compose.yml
└── .env.example
```

**Fluxo de dependências (hexagonal, dentro do módulo `customers`):**
```
Handler → Service (port input) → [Repository port output | Pipefy port output | EventPublisher port output]
                                          ↓                        ↓                        ↓
                                    SQLite adapter          Pipefy adapter          RabbitMQ adapter
```
As camadas internas (domain, service, ports) nunca importam adapters — as interfaces são
injetadas via construtor (dependency injection) no `cmd/main.go`. Ver `docs/adr/0001-*.md`.

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
- Go 1.25+
- Docker (opcional, para RabbitMQ / api-gateway / build das imagens)

### Modo simples (sem RabbitMQ)

```bash
cd customers
go run ./cmd/main.go
```

### Stack completa — customers + RabbitMQ + api-gateway (Docker Compose)

```bash
docker-compose up --build
```

RabbitMQ Management UI disponível em `http://localhost:15672` (guest/guest).
API acessível direto em `http://localhost:8080` (customers) ou pelo ponto de
entrada único em `http://localhost:8000/api/v1/customers/...` (api-gateway).

### Variáveis de Ambiente — customers

| Variável                | Padrão              | Descrição                              |
|--------------------------|---------------------|----------------------------------------|
| `PORT`                   | `8080`              | Porta HTTP                             |
| `DATABASE_DSN`           | `mundo_invest.db`   | Caminho do arquivo SQLite              |
| `RABBITMQ_URI`           | _(vazio)_           | URI AMQP (ex: `amqp://guest:guest@localhost:5672/`) |
| `PIPEFY_API_TOKEN`       | _(vazio)_           | Token Bearer da API Pipefy             |
| `PIPEFY_PIPE_ID`         | _(vazio)_           | ID do Pipe onde os cards serão criados |
| `API_AUTH_TOKEN`         | _(vazio)_           | Token Bearer exigido em `POST /clientes`. Vazio = endpoint sem autenticação (só aceitável em dev local — um aviso é logado no boot) |
| `PIPEFY_WEBHOOK_SECRET`  | _(vazio)_           | Segredo HMAC-SHA256 para validar o header `X-Pipefy-Signature` no webhook. Vazio = sem verificação de assinatura |
| `GIN_MODE`               | `release`\*         | \*O binário já força `release` quando a variável não está definida — só sobrescreva para `debug` em desenvolvimento |

### Variáveis de Ambiente — api-gateway

| Variável                | Padrão                    | Descrição                              |
|--------------------------|----------------------------|----------------------------------------|
| `PORT`                   | `8000`                     | Porta HTTP                             |
| `CUSTOMERS_SERVICE_URL`  | `http://localhost:8080`    | URL do serviço customers               |
| `ALLOWED_ORIGINS`        | _(vazio)_                  | Origens permitidas por CORS, separadas por vírgula. Vazio = nenhum cabeçalho CORS enviado |
| `RATE_LIMIT_PER_SECOND`  | `10`                       | Requisições por segundo permitidas por IP |
| `RATE_LIMIT_BURST`       | `20`                       | Rajada máxima do rate limiter por IP   |

Ambos os serviços expõem `GET /health` (liveness/readiness) e `GET /metrics`
(métricas Prometheus: contagem e latência de requisições HTTP).

---

## Rodando os Testes

Cada módulo (`customers`, `api-gateway`) roda seus testes a partir do seu
próprio diretório — é assim que o CI (`.github/workflows/ci.yml`) também roda.

```bash
cd customers

# Todos os testes (unitários + integração)
go test ./... -v

# Apenas testes unitários do service (mocks)
go test ./internal/core/service/... -v

# Apenas testes de integração (SQLite real :memory:)
go test ./internal/adapters/sqlite/... -v

# Apenas testes de domínio
go test ./internal/core/domain/... -v

# Apenas testes da camada HTTP (handlers + middleware de segurança)
go test ./internal/adapters/http/... -v
```

```bash
cd api-gateway
go test ./... -v   # proxy (stripPrefix), health, CORS, rate limit
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

Requer `Authorization: Bearer <API_AUTH_TOKEN>` quando essa variável está
configurada (ver [Variáveis de Ambiente](#variáveis-de-ambiente--customers)).

```bash
curl -X POST http://localhost:8080/clientes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_AUTH_TOKEN" \
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

**Resposta 401 (token ausente/incorreto, quando `API_AUTH_TOKEN` está configurado):**
```json
{ "error": "não autorizado" }
```

**Resposta 409 (e-mail já cadastrado):**
```json
{ "error": "e-mail já cadastrado" }
```

---

### POST /webhooks/pipefy/card-updated

Quando `PIPEFY_WEBHOOK_SECRET` está configurado, a requisição precisa do
header `X-Pipefy-Signature` com o HMAC-SHA256 (hex) do corpo bruto:

```bash
BODY='{
  "event_id": "evt_123",
  "card_id": "card_456",
  "cliente_email": "joao.silva@example.com",
  "timestamp": "2026-05-18T12:00:00Z"
}'
SIGNATURE=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$PIPEFY_WEBHOOK_SECRET" | sed 's/^.* //')

curl -X POST http://localhost:8080/webhooks/pipefy/card-updated \
  -H "Content-Type: application/json" \
  -H "X-Pipefy-Signature: $SIGNATURE" \
  -d "$BODY"
```

**Resposta 200:**
```json
{ "message": "webhook processado com sucesso" }
```

**Resposta 401 (assinatura ausente/inválida, quando `PIPEFY_WEBHOOK_SECRET` está configurado):**
```json
{ "error": "assinatura de webhook inválida" }
```

**Resposta 409 (event_id duplicado — idempotência):**
```json
{ "error": "evento já processado (event_id duplicado)" }
```

---

## Mutations GraphQL do Pipefy

O código completo está em `customers/internal/adapters/pipefy/client.go`, com links para a documentação oficial.  
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
# 1. Criar o Secret real a partir do template (NUNCA commitar o resultado —
#    k8s/secret.yaml já está no .gitignore)
cp k8s/secret.example.yaml k8s/secret.yaml
# edite k8s/secret.yaml com os valores reais, depois:
kubectl apply -f k8s/secret.yaml

# 2. Aplicar o restante dos manifests
kubectl apply -f k8s/configmap.yaml -f k8s/api-gateway-configmap.yaml \
  -f k8s/sqlite-pvc.yaml \
  -f k8s/rabbitmq-deployment.yaml -f k8s/rabbitmq-service.yaml \
  -f k8s/api-deployment.yaml -f k8s/api-service.yaml \
  -f k8s/api-gateway-deployment.yaml -f k8s/api-gateway-service.yaml \
  -f k8s/network-policies.yaml \
  -f k8s/ingress.yaml   # ajuste host/ingressClassName antes de aplicar

# Verificar pods
kubectl get pods

# Acompanhar logs
kubectl logs -f deployment/mundo-invest-api
kubectl logs -f deployment/mundo-invest-gateway

# Acessar via gateway (port-forward)
kubectl port-forward svc/mundo-invest-gateway 8000:80
```

Manifests disponíveis em `k8s/`:

| Arquivo | Descrição |
|---------|-----------|
| `configmap.yaml` | Variáveis de ambiente não-secretas do customers |
| `api-gateway-configmap.yaml` | Variáveis de ambiente não-secretas do api-gateway |
| `secret.example.yaml` | Template do Secret (Pipefy, `API_AUTH_TOKEN`, `PIPEFY_WEBHOOK_SECRET`) — copie para `secret.yaml` e preencha localmente; nunca commite o resultado. Ver `docs/adr/0006-*.md` |
| `sqlite-pvc.yaml` | PersistentVolumeClaim de 1Gi para o SQLite |
| `api-deployment.yaml` | Deployment do customers (1 réplica — ver `docs/adr/0003-*.md` sobre SQLite —, securityContext non-root, probes, resources) |
| `api-service.yaml` | ClusterIP na porta 80 |
| `api-gateway-deployment.yaml` | Deployment do api-gateway (2 réplicas, stateless, securityContext non-root) |
| `api-gateway-service.yaml` | ClusterIP na porta 80 |
| `rabbitmq-deployment.yaml` | RabbitMQ com management UI |
| `rabbitmq-service.yaml` | ClusterIP para AMQP (5672) e management (15672) |
| `network-policies.yaml` | Deny-all + allow-list explícita: internet → gateway → customers → rabbitmq |
| `ingress.yaml` | Exposição externa via Ingress Controller (ex.: ingress-nginx); ajuste `host`/`ingressClassName` |

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

---

## CI/CD

`.github/workflows/ci.yml` roda em todo push/PR para `main`: `gofmt`, `go vet`,
`golangci-lint` e `go test -race -cover`, para os módulos `customers` e
`api-gateway` de forma independente.

`.github/workflows/docker-publish.yml` builda e publica as duas imagens em
`ghcr.io/<owner>/mundo-invest-{customers,gateway}` a cada push em `main`,
tagueadas pelo SHA do commit (além de `latest`) — usa `GITHUB_TOKEN`, sem
segredos adicionais para configurar.

---

## Documentação adicional

- [`docs/adr/`](docs/adr/) — Architecture Decision Records: por que cada peça
  (arquitetura hexagonal, monorepo com Go workspaces, SQLite, RabbitMQ,
  api-gateway) é como é, e a decisão de gestão de segredos ainda em aberto.
- [`docs/LEARNING.md`](docs/LEARNING.md) — aprendizados técnicos registrados
  durante a auditoria de backend/infra deste repositório.
