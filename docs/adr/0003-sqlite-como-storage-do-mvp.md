# ADR-0003: SQLite como storage do MVP

## Status
Aceita para o MVP, com plano de substituição documentado

## Contexto
O serviço `customers` precisa de persistência transacional simples (clientes +
eventos de webhook) sem depender de infraestrutura externa para rodar
localmente ou em `docker-compose`.

## Decisão
Usar SQLite (`modernc.org/sqlite`, driver puro Go, sem CGO) via um `DB` que
roda migração idempotente (`CREATE TABLE IF NOT EXISTS`) no boot. O
`README.md` já documenta a migração planejada para Amazon RDS (Postgres) ou
DynamoDB em produção.

## Consequências
- **Positivo:** zero dependência externa para rodar o serviço; `CGO_ENABLED=0`
  mantém o Dockerfile simples (`FROM alpine` sem toolchain C).
- **Negativo (achado da auditoria de 2026-09-07):** `k8s/api-deployment.yaml`
  define `replicas: 2` sobre um único `sqlite-pvc` `ReadWriteOnce`. Múltiplos
  processos escrevendo concorrentemente no mesmo arquivo SQLite é incompatível
  com esse desenho de escala horizontal — o Deployment atual não é seguro para
  rodar com mais de 1 réplica. Isso precisa ser resolvido antes de produção:
  ou reduzir para `replicas: 1`, ou adiantar a migração para Postgres/RDS
  descrita no README.

## Atualização (2026-09-07)
`k8s/api-deployment.yaml` foi corrigido para `replicas: 1`, com comentário no
próprio manifest apontando para esta ADR. A migração para Postgres/RDS
continua sendo o gatilho correto para voltar a escalar horizontalmente —
nenhuma mudança de storage foi feita, só o desenho de deployment deixou de
estar inconsistente com o storage atual.

## Atualização (2026-09-07, parte 2) — limite medido *dentro* de uma réplica

Rodando o roteiro de verificação de performance
(`docs/testing/roteiro-verificacao-performance.md`), medimos que o problema
não era só "duas réplicas" — **mesmo com uma única réplica**, mais de 4
requisições de escrita simultâneas já causavam >98% de erro 500 por
contenção do SQLite (`SQLITE_BUSY`), porque `sqlite.NewDB` não configurava
`journal_mode` nem `busy_timeout`, e o pool do `database/sql` abria mais de
uma conexão física para o mesmo arquivo.

Corrigido no mesmo dia: `PRAGMA journal_mode=WAL`, `PRAGMA busy_timeout=5000`
e `db.SetMaxOpenConns(1)`. Revalidado com o mesmo teste — 100% de sucesso até
50 escritas simultâneas, com latência crescendo em vez de errar (p99 de
35ms a 5 workers até 314ms a 50 workers). Detalhes completos, com números de
antes/depois, em [`docs/testing/resultados-2026-09-07.md`](../testing/resultados-2026-09-07.md).

Isso **não substitui** a decisão desta ADR — SQLite com um único writer
serializado continua sendo um teto de throughput de escrita bem mais baixo
que Postgres/RDS teria. O que mudou é a forma como esse teto se manifesta:
antes, erro; agora, fila/latência. A migração de storage continua sendo o
gatilho certo se o volume de escrita justificar.
