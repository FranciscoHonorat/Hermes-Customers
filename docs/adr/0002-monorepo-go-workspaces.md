# ADR-0002: Monorepo com Go Workspaces (`go.work`)

## Status
Parcialmente superseded por [[0007-customers-como-modulo-proprio]] — o uso de
`go.work` para unir os três módulos continua válido; o que mudou foi
`customers` passar a ter `go.mod` próprio (ver ADR-0007).

## Contexto
`customers` e `api-gateway` precisam compartilhar o DTO de evento
(`shared.ClienteCreatedMessage`) publicado no RabbitMQ, sem publicar um módulo
Go privado nem duplicar a struct em cada serviço.

## Decisão (histórica — ver ADR-0007 para o estado atual)
Usar um workspace Go (`go.work` na raiz) referenciando três módulos: `.`
(módulo raiz, que na prática hospedava o serviço `customers` sob
`github.com/FranciscoHonorat/mundo-invest`), `./api-gateway` e `./shared`.
`customers` não tinha `go.mod` próprio — usava o módulo raiz diretamente.

## Consequências (na época)
- **Positivo:** builds e `go test ./...` locais resolviam os três módulos sem
  replace directives manuais ou publicação em registry.
- **Negativo:** a estrutura era assimétrica — `api-gateway` e `shared` eram
  módulos de verdade, mas `customers` não era. Corrigido pela ADR-0007.
- **Negativo:** cada `Dockerfile` de serviço copia o contexto de build
  completo da raiz do monorepo (`docker-compose.yml` usa `context: .` para o
  `customers` e `context: ./api-gateway` para o gateway) — sem
  `.dockerignore`, isso inflava a imagem e o cache de build ficava mais
  sensível a mudanças em serviços não relacionados. **Resolvido** em
  2026-09-07 com `.dockerignore` na raiz e em `api-gateway/`.
