# Resultados — Verificação de Performance (Hermes-Customers)

Execução do roteiro em `docs/testing/roteiro-verificacao-performance.md`,
Fases 1–5, no serviço `customers` + `api-gateway`. Todos os números abaixo
foram medidos, não estimados — comando exato incluído em cada linha para
reprodução.

## Ambiente

- **Data:** 2026-09-07
- **Onde:** local, via `docker-compose up --build` (imagens desta sessão de
  auditoria — non-root, `GIN_MODE=release`, `/metrics` habilitado)
- **Hardware:** máquina de desenvolvimento (não é hardware de produção — os
  números são um baseline comparável entre execuções na mesma máquina, não
  uma promessa de capacidade em qualquer ambiente)
- **Portas:** mapeadas em portas alternativas (18080/18000/15680) só para
  esta sessão, para não colidir com outro projeto (`Hermes-Movies`) já
  rodando na máquina — `docker-compose.yml` do repositório continua com as
  portas padrão (8080/8000/5672)

## Fase 1 — Throughput & Latência (`GET /health`)

| Cenário | Comando | Req/s | p50 | p95 | p99 | Success |
|---|---|---|---|---|---|---|
| customers direto | `hey -z 15s -c 50 http://localhost:18080/health` | **34.714** | 1.2ms | 3.2ms | 5.1ms | 100% |
| via api-gateway (rate limit desativado p/ medir throughput cru) | `hey -z 15s -c 50 http://localhost:18000/health` | **33.837** | 1.3ms | 3.4ms | 5.1ms | 100% |
| via api-gateway, config padrão (10 req/s, burst 20) | `hey -z 30s -q 100 -c 20 http://localhost:18000/health` | 1.999,88 solicitado | — | — | — | **0,53%** (59.681/60.000 = `429`) |

**Overhead do proxy:** ~2,5% (34.714 → 33.837 req/s) — o `api-gateway` não é
gargalo perceptível no caminho de leitura.

**O rate limiter funciona como configurado:** a 100 req/s sustentados de um
único IP contra um limite de 10 req/s + burst 20, 99,47% das requisições
tomaram `429` — validação de que `RateLimiter` (achado #6 da auditoria
original) está ativo e correto, não um bug. Para medir o teto real do
gateway é preciso subir `RATE_LIMIT_PER_SECOND`/`RATE_LIMIT_BURST` (como
feito na segunda linha) ou testar de múltiplos IPs simulados.

**Cross-check com `/metrics`:** contagem de `customers_http_requests_total`
e `gateway_http_requests_total` bateu com o total de requisições enviadas em
cada rodada (diferença de 1-2, das checagens manuais de `/health` feitas à
parte) — nenhuma requisição está sendo perdida silenciosamente entre o
cliente de carga e o handler.

## Fase 2 — Volume de Ingestão (RabbitMQ)

| Teste | Comando | Resultado |
|---|---|---|
| Publish direto na fila `clientes_criados` | `go run ./cmd/loadtest -n 10000 -uri amqp://guest:guest@localhost:15680/` | **10.000 eventos em 134ms → 74.579 eventos/s** |

Sem consumidor ativo (mede só a capacidade de publish do broker + client Go,
não o pipeline de processamento fim-a-fim). Ferramenta adicionada em
`customers/cmd/loadtest/main.go` — reutilizável para outros volumes/filas.

## Fase 3 — Memória & CPU (`docker stats`)

| Container | Repouso | Sob carga (escrita, ~40 req/s) | Após carga |
|---|---|---|---|
| `mundo-invest-api` (customers) | 22.6 MiB / 0.05% CPU | 23.2 MiB / 4.5% CPU | 21.7 MiB / 0% CPU |
| `mundo-invest-api-gateway` | 14.6 MiB / 0% CPU | 15.4 MiB / 1.8% CPU | 14.6 MiB / 0% CPU |
| `mundo-invest-rabbitmq` | 138.6 MiB / 0.1% CPU | 139.6 MiB / 188.7% CPU\* | 139.1 MiB / 50.7% CPU |

\*CPU multi-core (>100% possível). Memória de todos os três serviços ficou
estável — sem sinal de leak em nenhum dos testes rodados (curtos, de
segundos a ~1 minuto; não substitui um soak test de horas).

## Fase 4 — Endpoints específicos e limites de concorrência

### Segurança (endpoint protegido)

| Cenário | Esperado | Observado |
|---|---|---|
| `POST /clientes` sem `Authorization` | 401 | ✅ 401 |
| `POST /clientes` com token correto (1ª vez) | 201 | ✅ 201 |
| `POST /clientes` mesmo e-mail (2ª vez) | 409 | ✅ 409 |

### Throughput de escrita (baixa concorrência efetiva)

`vegeta attack -format=http -rate=40/1s -duration=10s -lazy` (400 clientes
com e-mail único, sem `-workers` fixo — vegeta usa ~1-2 workers simultâneos
nessa taxa):

- **400/400 sucesso (100%)**
- **Latência:** mean 5.7ms, p50 5.3ms, p95 8.1ms, **p99 11.5ms**, max 15.1ms

### ⚠️ Achado: falha catastrófica sob concorrência real de escrita

Testando com `-workers` fixo (concorrência forçada, não pautada por taxa),
usando e-mails únicos a cada rodada:

| Concorrência forçada | Requisições | Sucesso (201) | Falha (500) |
|---|---|---|---|
| 5 workers simultâneos | 201 | 3 (1,5%) | 197 (98%) |
| 20 workers simultâneos | 204 | 4 (2,0%) | 196 (96%) |
| 50 workers simultâneos | 205 | 2 (1,0%) | 198 (97%) |

**A partir de ~5 escritas verdadeiramente simultâneas, mais de 98% das
requisições falham com `500 {"error":"erro interno"}`.**

**Causa raiz (confirmada por eliminação, não só suposição):** o corpo do 500
é sempre o genérico do `default:` em `cliente_handler.go` — não é
`ErrEmailInvalido`/`ErrCampoObrigatorio`/`ErrPatrimonioInvalido` (400) nem
`ErrEmailDuplicado` (409, já validado acima como funcionando). A única causa
plausível restante é o driver SQLite (`modernc.org/sqlite`) retornando
`SQLITE_BUSY`/"database is locked" quando múltiplas conexões tentam
`INSERT` no mesmo arquivo ao mesmo tempo — `sqlite.NewDB` não configura
`journal_mode=WAL` nem `busy_timeout`, e o pool de conexões do
`database/sql` não está limitado a uma única conexão.

Isso **não contradiz** a auditoria original — é a mesma limitação já
registrada em [`docs/adr/0003-sqlite-como-storage-do-mvp.md`](../adr/0003-sqlite-como-storage-do-mvp.md)
(replicas fixadas em 1 por causa de single-writer), agora **medida
empiricamente dentro de uma única réplica**: o problema não é só "duas
réplicas escrevendo", é "mais de ~4 requisições simultâneas na mesma
réplica" já expõe o limite do SQLite sem tuning.

**Gap adicional descoberto:** o handler não loga o erro real antes de
retornar 500 (`cliente_handler.go`, branch `default`) — não foi possível
confirmar a mensagem exata do driver via `docker logs`, só inferir por
eliminação.

### ✅ Correção aplicada e revalidada (mesmo dia)

Em `sqlite.NewDB` (`customers/internal/adapters/sqlite/cliente_repository.go`):
`PRAGMA journal_mode=WAL`, `PRAGMA busy_timeout=5000` e `db.SetMaxOpenConns(1)`
— com uma única conexão, o `database/sql` enfileira as chamadas antes mesmo
de chegar no driver, então SQLite nunca vê duas escritas disputando o lock
ao mesmo tempo. Erros de `SQLITE_BUSY` que ainda assim ocorressem agora
mapeiam para `domain.ErrBancoIndisponivel` → **503** (não mais um 500
genérico), e o branch `default` dos handlers agora loga o erro real com
`slog.Error` antes de responder.

Reproduzindo exatamente o mesmo teste (200 clientes com e-mail único, direto
no `customers`, sem o gateway no meio):

| Concorrência forçada | Requisições enviadas | Sucesso (201) | Antes do fix |
|---|---|---|---|
| 5 workers simultâneos | 200 | **200 (100%)** | 1,5% |
| 20 workers simultâneos | 200 | **200 (100%)** | 2,0% |
| 50 workers simultâneos | 200 | **200 (100%)** | 1,0% |

**Trade-off honesto, esperado de serializar em 1 conexão:** a latência sobe
com a concorrência em vez de errar —

| Concorrência | p50 | p95 | p99 | max |
|---|---|---|---|---|
| 5 | 7.2 ms | 22.9 ms | 35.4 ms | 40.6 ms |
| 20 | 24.9 ms | 73.9 ms | 135.5 ms | 164.8 ms |
| 50 | 53.7 ms | 197.6 ms | 314.1 ms | 326.7 ms |

Isso é o comportamento correto para um MVP em SQLite: sob pico, as
requisições esperam a vez em vez de falhar. O teto real de throughput de
escrita continua sendo "quantas requisições cabem enfileiradas dentro do
`WriteTimeout` de 30s configurado no `http.Server`" — não foi testado até
esse limite nesta rodada. A correção de fundo continua sendo a migração
para Postgres/RDS já prevista na ADR-0003 quando o volume de escrita
justificar.

Testes automatizados adicionados: `TestIntegration_EscritasConcorrentes_NaoFalham`
(sqlite, 20 goroutines concorrentes) e `TestCriarCliente_BancoIndisponivel_Retorna503`
/ `TestCardUpdated_BancoIndisponivel_Retorna503` (handlers).

## Resumo para o README

```
| Métrica                          | Valor              |
|-----------------------------------|--------------------|
| Throughput leitura (customers)    | 34.714 req/s       |
| Throughput leitura (via gateway)  | 33.837 req/s       |
| P99 leitura                       | 5.1 ms             |
| Throughput escrita (baixa conc.)  | 100% sucesso, p99 11.5 ms |
| Concorrência de escrita (após fix WAL+busy_timeout) | 100% sucesso até 50 workers simultâneos (antes: 98% de falha a partir de 5) |
| Ingestão RabbitMQ                 | 74.579 eventos/s   |
| Memória em repouso (customers)    | 22.6 MiB           |
```
