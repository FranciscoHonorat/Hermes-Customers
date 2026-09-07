# ADR-0005: `api-gateway` como reverse proxy dedicado

## Status
Aceita (retroativa)

## Contexto
O plano de produção (README, seção "Visão de Produção (AWS)") prevê API
Gateway + Lambda por endpoint. Localmente, o equivalente é um serviço Go
próprio que concentra o ponto de entrada HTTP e futuramente hospedaria
autenticação, rate limiting e roteamento entre múltiplos serviços internos
(hoje só `customers`).

## Decisão
Criar `api-gateway` (Gin + `httputil.ReverseProxy`) expondo
`/api/v1/customers/*`, que reescreve o path (remove o prefixo) e encaminha
para `CUSTOMERS_SERVICE_URL`. `customers` continua com seu próprio `/health`,
`/clientes` e `/webhooks/...` expostos diretamente também.

## Consequências
- **Positivo:** ponto único de entrada versionado (`/api/v1/...`), separado da
  API interna do serviço `customers`.
- **Negativo (achado da auditoria de 2026-09-07):** o gateway hoje é só um
  proxy — sem autenticação, rate limiting, CORS ou agregação de
  observabilidade — e **não tem nenhum manifest em `k8s/`**. O serviço
  documentado como "ponto de entrada único" da Hermes não é implantável no
  cluster no estado atual do repositório; só existe via `docker-compose`.
- Nota: o diff em andamento em `reverse_proxy.go` (migração de `Director` para
  o hook `Rewrite` do `httputil.ReverseProxy`, API recomendada desde Go 1.20)
  é a direção correta e deveria ser commitado junto de um teste de regressão
  para o comportamento de `stripPrefix` (ver `LEARNING.md`).

## Atualização (2026-09-07)
Todos os itens do achado acima foram resolvidos: `api-gateway` ganhou CORS
configurável, rate limiting por IP, request-id e métricas Prometheus
(`internal/middleware/`), e manifests próprios em `k8s/` (`api-gateway-deployment.yaml`,
`api-gateway-service.yaml`, `api-gateway-configmap.yaml`), além de Ingress e
NetworkPolicy cobrindo o caminho internet → gateway → customers. O diff do
`reverse_proxy.go` foi commitado com testes de regressão para `stripPrefix`
(`reverse_proxy_test.go`). Autenticação de borda (verificação de identidade do
chamador) ainda não existe no gateway — hoje é feita pelo `customers` via
`API_AUTH_TOKEN` (ver ADR-0001 e o handler em `customers/internal/adapters/http/middleware/auth.go`).
