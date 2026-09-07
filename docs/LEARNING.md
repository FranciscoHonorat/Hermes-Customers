# Learnings — Auditoria Backend & Infra/DevOps (2026-09-07)

Registro dos aprendizados que surgiram ao auditar `customers` + `api-gateway`.
Não é um changelog do produto — é o que vale carregar para o próximo serviço
Go/k8s deste time.

## Go / backend

- **`httputil.ReverseProxy.Director` está deprecated desde Go 1.20** em favor
  do hook `Rewrite` (`ProxyRequest.SetURL` + `r.Out`/`r.In`). O diff que já
  estava em andamento em `reverse_proxy.go` migra exatamente para isso — é a
  direção certa, mas ficou sem teste cobrindo o `stripPrefix`. Lição: toda
  migração de API deprecated merece um teste de regressão no mesmo commit,
  não só a migração.
- **`gin.Default()` roda em modo debug por padrão.** Sem `GIN_MODE=release`
  setado em algum lugar (env, Dockerfile, ConfigMap), toda instância em
  produção roda com logging verboso e comportamento de debug do Gin. Isso é
  fácil de esquecer porque o código "funciona" normalmente — só aparece como
  problema em produção.
- **`binding:"required"` do Gin trata zero-value como ausente.** Um campo
  `float64` com `binding:"required"` rejeita `0` como se o campo não tivesse
  sido enviado. Vale checar isso sempre que um campo numérico legitimamente
  pode ser zero (aqui, `valor_patrimonio: 0` seria um caso de negócio válido
  e é rejeitado como erro 400).
- **Erro de driver SQL vazando como 500.** Quando um adapter (`sqlite.Salvar`)
  só faz `fmt.Errorf("...: %w", err)` em cima do erro cru do driver, a camada
  de service precisa mapear explicitamente os erros conhecidos (ex.: `UNIQUE
  constraint failed`) para erros de domínio — senão a camada HTTP não tem como
  diferenciar "conflito" de "erro interno".

## Kubernetes

- **SQLite (ou qualquer storage single-writer) não combina com
  `replicas > 1` + PVC `ReadWriteOnce`.** É um anti-padrão comum ao promover
  um protótipo local (que roda em 1 processo) direto para um Deployment com
  múltiplas réplicas sem revisitar a camada de storage.
- **Um serviço "documentado como implantável" nem sempre tem manifests.** O
  `api-gateway` é descrito no README como o ponto de entrada da arquitetura,
  mas não existe nenhum YAML para ele em `k8s/`. Vale sempre conferir
  `kubectl apply -f k8s/ --dry-run=client` (ou equivalente) contra a lista de
  serviços do `docker-compose.yml` como checklist de paridade.

## DevOps

- **Ausência de `.dockerignore` é um risco de segurança, não só de tamanho de
  imagem.** Com `COPY . .` e sem `.dockerignore`, qualquer `.env` local
  (mesmo estando no `.gitignore`, que só protege o Git) entra no contexto de
  build e pode acabar dentro da imagem.
- **Sem CI, `gofmt`/`go vet`/testes são best-effort.** Rodar manualmente
  durante esta auditoria (`gofmt -l .`, `go vet ./...`) já encontrou 2
  arquivos fora do padrão — nada bloqueava isso antes do merge.

## Processo desta auditoria

- Auditoria feita por leitura de código + execução local de `gofmt`,
  `go vet`, `go build` e `go test ./...` — sem acesso a cluster real, então
  os achados de k8s (réplicas × PVC, ausência de manifest do gateway) são
  verificados pela definição declarativa, não por comportamento observado em
  runtime.
- ADRs em `docs/adr/` foram escritas de forma retroativa para as decisões já
  implementadas (0001–0005) e uma proposta em aberto (0006, gestão de
  segredos) — útil como formato para não perder o "porquê" de decisões que já
  existem no código mas nunca foram escritas em lugar nenhum.

## Atualização (2026-09-07): os 28 achados foram corrigidos na mesma sessão

- Validação real, não só leitura de código: as duas imagens Docker foram
  reconstruídas e rodadas (`docker build` + `docker run`) para confirmar
  usuário non-root, escrita no volume, `/health`, `/metrics` e a autenticação
  por API key end-to-end — não bastava o código compilar, precisava rodar.
- **`go get` de uma dependência nova pode subir o `go` directive do
  `go.mod` sozinho** (aqui, adicionar `client_golang` empurrou o mínimo de
  1.22 para 1.25) — isso quebra silenciosamente o build da imagem Docker se
  o `Dockerfile` fixa uma tag de builder mais antiga (`golang:1.22-alpine`).
  Sempre rebuildar a imagem depois de qualquer `go get`/`go mod tidy`.
- Ao mover `customers` para um `go.mod` próprio (ADR-0007), o fato de os
  imports internos já usarem o caminho completo
  (`.../mundo-invest/customers/internal/...`) significou zero mudança de
  import — só o `go.mod` mudou de lugar e de nome. Vale desenhar module paths
  já pensando nisso desde o início de um monorepo, mesmo que o módulo raiz
  ainda não exista.
- Testar a stack completa via `docker-compose up` nem sempre é viável no
  ambiente de quem está revisando: uma imagem `rabbitmq:3-management` rodada
  manualmente falhou por uma permissão de volume anônimo específica da sandbox
  usada nesta sessão, sem relação com o código alterado. Nesses casos, validar
  cada peça isoladamente (testes Go com `httptest`, container isolado por
  serviço) é mais confiável do que insistir na integração completa.

## Atualização (2026-09-07, parte 2): rodar o roteiro de performance achou um bug real

- **`go-sqlite`/`database/sql` sem tuning é uma armadilha clássica que a
  auditoria de código não pega, só carga real pega.** `go vet`, testes
  unitários com mocks e até os testes de integração com SQLite `:memory:`
  (que rodam sequencialmente) não têm como revelar contenção de escrita
  concorrente — só apareceu forçando `vegeta -workers=N` contra o serviço de
  verdade. Lição: para qualquer serviço com estado, pelo menos um teste de
  concorrência forçada (não só de taxa) deveria fazer parte do roteiro
  padrão, não ser uma descoberta acidental.
- **A correção de SQLite+`database/sql` mais citada por aí
  (`journal_mode=WAL` + `busy_timeout` + `SetMaxOpenConns(1)`) realmente
  resolve** — de 98% de falha para 100% de sucesso no mesmo teste, sem
  mudar nenhuma outra parte do sistema. O trade-off (latência sobindo com a
  concorrência, já que tudo serializa numa conexão) é o comportamento
  correto a se esperar, não um efeito colateral a esconder.
- **Um `default: c.JSON(500, "erro interno")` sem logar o erro original é
  uma cegueira de produção esperando para acontecer.** Só descobrimos a
  causa raiz por eliminação (nenhum outro erro de domínio bate com o
  sintoma) — se o handler tivesse logado o erro desde o início, a
  investigação teria sido imediata em vez de indireta.
