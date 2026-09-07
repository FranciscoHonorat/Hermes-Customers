# ADR-0007: `customers` como módulo Go próprio

## Status
Aceita (2026-09-07) — supersede parcialmente [[0002-monorepo-go-workspaces]]

## Contexto
A auditoria de backend/infra de 2026-09-07 (achado #27) identificou que
`customers` era o único dos três componentes do monorepo (`customers`,
`api-gateway`, `shared`) sem `go.mod` próprio — ele reaproveitava o `go.mod`
da raiz do repositório, com módulo nomeado `github.com/FranciscoHonorat/mundo-invest`
(sem sufixo `/customers`). Isso deixava a estrutura do monorepo assimétrica e
dificultava tratar `customers` como uma unidade de build/versionamento
independente (ex.: builds e pipelines de CI por módulo, como já acontecia com
`api-gateway`).

## Decisão
Mover `go.mod`/`go.sum` da raiz para `customers/go.mod` e `customers/go.sum`,
renomeando o módulo para `github.com/FranciscoHonorat/mundo-invest/customers`.
Como os pacotes internos já usavam esse caminho completo nos imports (ex.:
`.../mundo-invest/customers/internal/core/domain`), **nenhum import precisou
mudar** — só o `go.mod` e a localização física do arquivo. `go.work` passou a
apontar para `./customers` em vez de `.`.

A raiz do repositório deixou de ser um módulo Go: `go build ./...` agora roda
por módulo (`cd customers && go build ./...`, igual a `api-gateway`), e é
assim que `.github/workflows/ci.yml` também roda (matriz `[customers, api-gateway]`).

## Consequências
- **Positivo:** os três componentes do monorepo seguem agora o mesmo padrão
  estrutural — cada um com seu `go.mod`, testável e buildável de forma
  independente.
- **Positivo:** nenhuma mudança de import necessária, graças ao module path
  já ter o sufixo `/customers` desde o início.
- **Negativo:** `customers/Dockerfile` ficou um pouco mais específico — copia
  `customers/go.mod`/`customers/go.sum` para dentro de `customers/` na imagem
  antes de rodar `go mod download`, já que o contexto de build continua sendo
  a raiz do monorepo (necessário para o `COPY . .` trazer `go.work` e
  `shared/`, que `customers` importa). `api-gateway/Dockerfile` não precisou
  dessa adaptação, pois não depende de `shared`.
- **Negativo:** comandos como `go build ./...` na raiz do repositório não
  funcionam mais (a raiz não é módulo) — é preciso rodar de dentro de
  `customers/` ou `api-gateway/`, ou usar o caminho de import completo em
  modo workspace (`go build github.com/.../mundo-invest/customers/...`).
  Documentado no README.
