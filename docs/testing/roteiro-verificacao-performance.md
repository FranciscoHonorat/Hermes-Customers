# Roteiro de Verificação de Performance — Portfolio

**Francisco — Portfolio Verification Workflow (adaptado)**

Versão adaptada do roteiro original: **Fase 0 (deploy) foi removida** — assume-se
que o serviço já está rodando em algum lugar acessível (local via
`docker-compose up`, ou já implantado). O roteiro começa direto na medição.

Este documento é **agnóstico de projeto**: os comandos usam `${BASE_URL}` como
variável, e cada fase que depende da stack (ingestão, query) traz uma variante
em Go e uma em Node/TS. Copie este arquivo para qualquer outro repositório
seu (`Hermes-Movies`, `Sisifo-Order-Flow-Studing`,
`Hermes-OrderFlow-Analytics-Platform`, etc.) e ajuste só os exemplos marcados
com `<!-- adaptar -->`.

O exemplo "vivo" usado nas seções abaixo é o **Hermes-Customers**
(`customers` + `api-gateway`, Go/Gin, SQLite, RabbitMQ) — porque é o projeto
que está rodando localmente agora e já expõe `/health` e `/metrics`.

---

## Pré-condição (substitui a Fase 0)

Antes de rodar qualquer coisa abaixo, confirme só isto:

```bash
export BASE_URL="http://localhost:8000"   # api-gateway; ou :8080 para customers direto

curl -sf "$BASE_URL/health" && echo " -> serviço respondendo"
```

Se isso falhar, **pare aqui** — não é um problema de performance, é um
problema de ambiente. No caso do Hermes-Customers:

```bash
docker-compose up --build -d
docker-compose ps        # tudo "healthy" / "running"?
docker-compose logs -f api api-gateway
```

Com o `/health` respondendo, siga para a Fase 1.

---

## FASE 1: TESTES DE CARGA — Throughput & Latência

### Objetivo

Medir quantas requisições por segundo seu sistema aguenta, e com qual latência
(p50/p95/p99) — não só a média, que esconde os piores casos.

### 1.1 — Ferramentas

Três opções, da mais simples à mais completa. Não precisa instalar as três —
escolha uma e siga.

```bash
# Opção A: Apache Bench — já vem em muitas distros, simples, só dá média/max
sudo apt-get install apache2-utils      # Ubuntu/Debian
brew install httpd                      # macOS

# Opção B: hey — binário único em Go, dá percentis, fácil se você já tem Go
go install github.com/rakyll/hey@latest

# Opção C: vegeta — o mais completo (distribuição de latência, relatórios)
go install github.com/tsenart/vegeta@latest
```

Se não quiser instalar nada agora, use o fallback com `curl` + `bash` na
seção 1.4 — dá menos detalhe, mas não bloqueia o teste.

### 1.2 — Teste simples (Apache Bench)

```bash
# 1000 requisições, 10 em paralelo
ab -n 1000 -c 10 "$BASE_URL/health" | tee resultado-load-health.txt

# Mais intenso: 5000 requisições, 50 em paralelo, num endpoint real
ab -n 5000 -c 50 "$BASE_URL/api/v1/customers/health" | tee resultado-load-gateway.txt
```

Output esperado (o que importa extrair):

```
Requests per second:    1234.56 [#/sec] (mean)
Time per request:       8.109 [ms] (mean)
Failed requests:        0
```

### 1.3 — Teste com percentis (hey ou vegeta)

**Com `hey`** (mais simples de ler):

```bash
hey -z 30s -q 100 -c 20 "$BASE_URL/health"
```

```
Summary:
  Total:        30.0123 secs
  Requests/sec: 98.7234

Latency distribution:
  50% in 0.0142 secs
  95% in 0.0298 secs
  99% in 0.0451 secs
```

**Com `vegeta`** (mais controle sobre múltiplos endpoints numa mesma sessão):

```bash
cat > requisicoes.txt <<EOF
GET ${BASE_URL}/health
GET ${BASE_URL}/metrics
EOF
# Para endpoints protegidos (POST /clientes com API_AUTH_TOKEN configurado),
# use o formato de header do vegeta:
cat > requisicoes-clientes.txt <<EOF
POST ${BASE_URL}/api/v1/customers/clientes
Content-Type: application/json
Authorization: Bearer ${API_AUTH_TOKEN}
@payload-cliente.json
EOF

cat requisicoes.txt | vegeta attack -duration=30s -rate=100 -timeout=10s \
  | vegeta report
```

```
Requests      [total, rate, throughput]    3000, 100.00, 99.50
Latencies     [min, mean, 50, 95, 99, max] 45.2ms, 156.4ms, 142ms, 285ms, 450ms, 2.1s
Success       [ratio]                      99.80%
```

### 1.4 — Fallback sem instalar nada (curl + bash)

```bash
#!/bin/bash
# scripts/load-test-simples.sh
BASE_URL="${BASE_URL:-http://localhost:8000}"
N="${1:-100}"

echo "=== TESTE DE CARGA SIMPLES — $N requisições paralelas ==="
time (
  for i in $(seq 1 "$N"); do
    curl -s -o /dev/null -w "%{http_code} %{time_total}s\n" "$BASE_URL/health" &
  done
  wait
) 2>&1 | tee resultado-curl-loop.txt

echo "--- Distribuição de status codes ---"
grep -oE "^[0-9]{3}" resultado-curl-loop.txt | sort | uniq -c
```

```bash
chmod +x scripts/load-test-simples.sh
./scripts/load-test-simples.sh 200
```

### 1.5 — Cross-check com as métricas do próprio serviço

Se o seu serviço já expõe `/metrics` (Prometheus) — como o Hermes-Customers
depois desta sessão de auditoria — compare o que a ferramenta de carga
externa mediu com o que a aplicação registrou internamente. Isso pega
diferenças entre "latência vista pelo cliente" (inclui rede/proxy) e
"latência vista pelo handler":

```bash
# Antes do teste
curl -s "$BASE_URL/metrics" | grep customers_http_request_duration_seconds_count > antes.txt

# ... rode o teste de carga (1.2/1.3) ...

# Depois do teste
curl -s "$BASE_URL/metrics" | grep customers_http_request_duration_seconds_count > depois.txt
diff antes.txt depois.txt
```

Se a diferença de contagem não bater com o número de requisições enviadas,
algo está sendo derrubado antes de chegar no handler (rate limiter do
gateway, timeout, conexão recusada) — informação tão importante quanto o
throughput em si.

---

## FASE 2: TESTES DE VOLUME DE DADOS

### Objetivo

Medir quantos registros seu sistema processa em lote — ingestão de eventos,
agregações de query — não só requisições HTTP avulsas.

Escolha a variante conforme o projeto:

### 2.1a — Ingestão via fila (Go + RabbitMQ) <!-- Hermes-Customers, Hermes-Movies -->

Já implementado e testado em `customers/cmd/loadtest/main.go` (não é
pseudocódigo — é a ferramenta real usada em
[`resultados-2026-09-07.md`](resultados-2026-09-07.md)):

```bash
cd customers
go run ./cmd/loadtest -n 10000 -uri amqp://guest:guest@localhost:5672/
```

Se for adaptar para outro projeto Go sem essa ferramenta pronta, o núcleo é
simples — publish em loop medindo o tempo total:

```go
func main() {
	uri := "amqp://guest:guest@localhost:5672/"
	conn, err := amqp.Dial(uri)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	ch, _ := conn.Channel()
	defer ch.Close()

	const total = 10_000
	start := time.Now()

	for i := 0; i < total; i++ {
		body, _ := json.Marshal(map[string]any{
			"cliente_id": i,
			"email":      fmt.Sprintf("teste%d@example.com", i),
			"status":     "Aguardando Análise",
		})
		ch.PublishWithContext(context.Background(), "", "clientes_criados", false, false,
			amqp.Publishing{ContentType: "application/json", Body: body})
	}

	elapsed := time.Since(start)
	fmt.Printf("Publicados %d eventos em %s (%.0f eventos/s)\n",
		total, elapsed, float64(total)/elapsed.Seconds())
}
```

### 2.1b — Ingestão via fila (Node + Redis Streams) <!-- Hermes-OrderFlow-Analytics-Platform -->

```javascript
// scripts/loadtest/ingest-test.js
const redis = require('redis');
const client = redis.createClient({ url: process.env.REDIS_URL });

async function testIngest() {
  await client.connect();
  const streamKey = 'hermes:events';
  const total = 10_000;

  console.time(`Ingest ${total} events`);
  for (let i = 0; i < total; i++) {
    await client.xAdd(streamKey, '*', {
      timestamp: new Date().toISOString(),
      level: 'info',
      message: `Test event ${i}`,
    });
  }
  console.timeEnd(`Ingest ${total} events`);

  console.log('Total no stream:', await client.xLen(streamKey));
  await client.quit();
}

testIngest().catch(console.error);
```

```bash
node scripts/loadtest/ingest-test.js
```

**Métrica extraída (qualquer variante):** "10.000 eventos publicados em 2.3s
= ~4.300 eventos/segundo" — anote o número real, não estime.

### 2.2 — Teste de query/agregação

**SQLite (Hermes-Customers):**

```bash
# Popule antes com um script de seed, depois:
sqlite3 mundo_invest.db ".timer on" \
  "SELECT status, COUNT(*), AVG(valor_patrimonio) FROM clientes GROUP BY status;"
```

**Postgres/TimescaleDB (outros projetos):**

```javascript
// scripts/loadtest/query-test.js
const { Pool } = require('pg');
const pool = new Pool({ connectionString: process.env.DATABASE_URL });

async function testQuery() {
  console.time('Agregação 24h');
  const result = await pool.query(`
    SELECT DATE_TRUNC('minute', timestamp) AS minute, COUNT(*) AS count
    FROM events
    WHERE timestamp > NOW() - INTERVAL '24 hours'
    GROUP BY minute ORDER BY minute DESC
  `);
  console.timeEnd('Agregação 24h');
  console.log('Linhas retornadas:', result.rows.length);
  await pool.end();
}

testQuery().catch(console.error);
```

**Métrica extraída:** "Agregação de N registros em X ms".

---

## FASE 3: TESTES DE MEMÓRIA & CPU

### 3.1 — Perfil básico (qualquer stack, via Docker)

Independente da linguagem, `docker stats` já dá memória/CPU reais do
container, sem instrumentar nada:

```bash
docker stats --no-stream mundo-invest-api mundo-invest-api-gateway mundo-invest-rabbitmq
```

Rode uma vez em repouso, e uma vez durante um teste de carga (Fase 1) num
segundo terminal, para comparar:

```bash
# Terminal 1
docker stats mundo-invest-api mundo-invest-api-gateway

# Terminal 2, simultaneamente
hey -z 60s -q 100 -c 20 "$BASE_URL/health"
```

Anote: memória em repouso, memória no pico da carga, memória alguns segundos
depois de parar a carga (se não volta a cair, é sinal de possível leak).

### 3.2 — Perfil profundo em Go (pprof)

Serviços Go se beneficiam de um profile real em vez de só `ps`/`docker stats`.
Se o seu serviço ainda não expõe pprof, é uma linha:

```go
import _ "net/http/pprof"
// e um listener HTTP interno, ex: go func() { http.ListenAndServe(":6060", nil) }()
```

Com isso disponível:

```bash
# CPU profile durante 30s de carga
go tool pprof -seconds=30 http://localhost:6060/debug/pprof/profile

# Heap no momento
go tool pprof http://localhost:6060/debug/pprof/heap
```

> Nota: isso é instrumentação nova, não algo que o Hermes-Customers já expõe
> hoje — trate como item de implementação antes de rodar esta subfase, não
> como bloqueio para as fases 1/2.

### 3.3 — Node.js (`--max-old-space-size` + `ps`)

```bash
node --max-old-space-size=4096 seu-app.js &
APP_PID=$!
sleep 120
ps -o pid,%cpu,%mem,rss,vsz -p $APP_PID
kill $APP_PID
```

---

## FASE 4: TESTE DE ENDPOINTS ESPECÍFICOS

Repita a Fase 1 por endpoint, não só num agregado — throughput varia muito
entre um `GET /health` (quase grátis) e um `POST` que grava no banco e
publica na fila.

**Exemplo concreto — Hermes-Customers:**

```bash
# 1. Health (deve ser o mais rápido — baseline)
hey -z 15s -c 50 "$BASE_URL/health"

# 2. Auth: confirme 401 sem token e 201/409 com token (rápido, não é teste de carga)
curl -s -o /dev/null -w "%{http_code}\n" -X POST "$BASE_URL/api/v1/customers/clientes" \
  -H "Content-Type: application/json" -d '{"cliente_nome":"T","cliente_email":"t@x.com","tipo_solicitacao":"X","valor_patrimonio":1}'
```

> ⚠️ **Lição aprendida rodando isto de verdade:** `hey -D payload.json` manda
> o **mesmo corpo** em toda requisição. Num endpoint com constraint de
> unicidade (como `cliente_email`), a 1ª chamada dá 201 e todas as seguintes
> dão 409 — você mede o caminho de conflito, não o de escrita bem-sucedida.
> Para medir throughput de escrita de verdade, gere um payload por
> requisição com e-mail único e use `vegeta` (que lê um arquivo de targets
> com um corpo por linha), não `hey`:

```bash
# Gere N payloads com e-mail único (RUN evita colidir com dados de execuções anteriores)
RUN=$(date +%s)
for i in $(seq 1 400); do
  echo "{\"cliente_nome\":\"Carga $i\",\"cliente_email\":\"carga${RUN}_${i}@example.com\",\"tipo_solicitacao\":\"Abertura\",\"valor_patrimonio\":$((RANDOM % 500000))}" > body_${i}.json
done
for i in $(seq 1 400); do
  printf "POST %s/api/v1/customers/clientes\nContent-Type: application/json\nAuthorization: Bearer %s\n@body_%d.json\n\n" "$BASE_URL" "$API_AUTH_TOKEN" "$i"
done > targets-clientes.txt

vegeta attack -format=http -rate=40/1s -duration=10s -lazy < targets-clientes.txt > results.bin
vegeta report results.bin
```

### 4.1 — Encontrando o limite real de concorrência de escrita

`-rate=N/1s` sem `-workers` deixa o vegeta escolher a concorrência — em taxas
baixas isso pode significar só 1-2 requisições simultâneas de verdade, o que
esconde problemas de contenção (lock de banco, connection pool). Para achar
o limite real, force a concorrência com `-workers` e suba o valor:

```bash
for CONC in 5 20 50; do
  echo "=== concorrência forçada: $CONC ==="
  vegeta attack -format=http -rate=0 -duration=5s -workers=$CONC -max-workers=$CONC -lazy \
    < targets-clientes.txt | vegeta report -type=text | grep -E "Success|Status Codes"
done
```

Se o `Success` ratio despencar a partir de algum nível de `$CONC` (foi
exatamente o que aconteceu no Hermes-Customers — >98% de falha já em 5
workers simultâneos, por contenção de escrita do SQLite — ver
[`resultados-2026-09-07.md`](resultados-2026-09-07.md)), **isso é o número
mais importante do teste inteiro**: não "quantos req/s aguenta", mas "a
partir de quantas escritas simultâneas ele quebra".

```bash
# 3. Webhook (idempotência — segunda chamada com o mesmo event_id deve retornar 409)
echo '{"event_id":"evt_loadtest_1","card_id":"card_1","cliente_email":"teste@example.com","timestamp":"2026-05-18T12:00:00Z"}' > payload-webhook.json
curl -s -X POST -H "Content-Type: application/json" -d @payload-webhook.json "$BASE_URL/api/v1/customers/webhooks/pipefy/card-updated"
curl -s -o /dev/null -w "%{http_code}\n" -X POST -H "Content-Type: application/json" -d @payload-webhook.json "$BASE_URL/api/v1/customers/webhooks/pipefy/card-updated"
```

<!-- adaptar: troque os payloads/paths pelos endpoints reais do projeto que você está testando -->

Guarde cada resultado num arquivo (`resultado-health.txt`,
`resultado-criar-cliente.txt`, ...) — a Fase 5 consome esses arquivos.

---

## FASE 5: COMPILAR RESULTADOS

Não interprete os números de cabeça — preencha esta tabela lendo direto dos
arquivos salvos nas fases 1 e 4:

```markdown
## Performance Metrics (Verificado via Load Test)

### Throughput
- **GET /health:**            _____ req/s
- **POST /clientes:**         _____ req/s
- **Ingestão (fila):**        _____ eventos/s

### Latência (POST /clientes, sob carga)
- **P50:** _____ ms
- **P95:** _____ ms
- **P99:** _____ ms
- **Max:** _____ ms

### Confiabilidade
- **Success rate:** _____ %
- **Status codes observados:** _____ (ex.: 429 do rate limiter contam aqui)

### Recursos (docker stats)
- **Memória em repouso:** _____ MB
- **Memória no pico:**    _____ MB
- **CPU no pico:**        _____ %

### Volume
- **N eventos ingeridos em:** _____ s
- **Query de agregação (N linhas) em:** _____ ms

### Capturado em
- Data: <!-- YYYY-MM-DD -->
- Projeto/commit: <!-- ex.: Hermes-Customers @ <sha> -->
- Ferramenta: <!-- hey / vegeta / ab -->
- Ambiente: <!-- local via docker-compose / cluster k8s / produção -->
- Comando exato usado: <!-- cole o comando, não parafraseie -->
```

---

## FASE 6: DOCUMENTAÇÃO NO README DO PROJETO TESTADO

```markdown
## Performance

Testado com `hey`/`vegeta` em `<data>`, ambiente local via `docker-compose`.
Detalhes e comandos exatos em `docs/testing/resultados-<data>.md`.

| Métrica                  | Valor       | Comando |
|---------------------------|-------------|---------|
| Throughput (/health)      | X req/s     | `hey -z 15s -c 50 $BASE_URL/health` |
| P99 latência (/health)    | Y ms        | idem |
| Throughput escrita        | Z% sucesso, p99 W ms | `vegeta attack -rate=N/1s -duration=Ns` |
| Limite de concorrência    | N simultâneas antes de degradar | `vegeta attack -workers=N` (seção 4.1) |
| Ingestão (fila)           | M eventos/s | `go run ./cmd/loadtest -n 10000` |
| Memória (repouso)         | K MB        | `docker stats` |

Reproduzir: `docs/testing/roteiro-verificacao-performance.md`.
```

**Exemplo real preenchido:** ver a seção "Performance" no `README.md` do
Hermes-Customers e o detalhamento completo em
[`resultados-2026-09-07.md`](resultados-2026-09-07.md) — inclui um achado de
limite de concorrência (não só números "bonitos"), que é exatamente o tipo
de honestidade que a Regra de Ouro abaixo pede.

---

## FASE 7: ROTEIRO DIA A DIA (sem a etapa de deploy)

### Dia 1 — Baseline
- [ ] `docker-compose up --build -d` (ou equivalente do projeto)
- [ ] `curl $BASE_URL/health` respondendo
- [ ] `hey`/`vegeta`/`ab` instalado
- [ ] Rodar Fase 1.2 (Apache Bench simples) e salvar output

### Dia 2 — Carga com percentis
- [ ] Rodar Fase 1.3 (`hey`/`vegeta`) por 30s em pelo menos 2 endpoints
- [ ] Cross-check com `/metrics` (Fase 1.5), se o projeto expõe
- [ ] Salvar todos os outputs em arquivos versionáveis

### Dia 3 — Volume e recursos
- [ ] Fase 2: teste de ingestão (10k registros)
- [ ] Fase 2: teste de query/agregação
- [ ] Fase 3: `docker stats` em repouso e sob carga

### Dia 4 — Endpoints específicos + compilação
- [ ] Fase 4: repetir carga por endpoint real do projeto
- [ ] Fase 5: preencher a tabela de métricas com números reais

### Dia 5 — Documentação e validação
- [ ] Fase 6: colar tabela no README do projeto testado
- [ ] Re-rodar **um** teste para confirmar que o número não foi digitado errado
- [ ] Atualizar CV/portfolio só com o que foi medido

---

## REGRA DE OURO

**✅ Coloque no README:**
```
Latency: 89ms (p99) — hey, 30s @ 100 req/s, 20 concurrent, docker-compose local
```

**❌ Não coloque:**
```
Suporta milhões de eventos (nunca testado)
Usado em produção por empresas (não é verdade)
```

**Se o teste não rodou, o número não entra.** Isso vale tanto para "está
rápido" quanto para "está devagar" — um número baixo e honesto ainda prova
que você sabe medir; um número inventado não prova nada e quebra confiança
se alguém pedir para reproduzir.

---

## Dúvidas Comuns

**E se o projeto for lento?**
Melhor 500 req/s verificado do que 10k fabricado.

**Preciso de um número alto para "parecer bom"?**
Não — a métrica prova que você sabe medir performance, não que o projeto é o
mais rápido do mercado.

**E se o serviço cair durante o teste de carga?**
Ótimo achado: agora você sabe exatamente onde está o limite, e isso vira
outro item de portfólio (identificou o gargalo, sabe o porquê).

---

## Próximos passos

Este documento é reutilizável — para aplicá-lo de fato, falta decidir:

1. **Qual projeto primeiro?** (Hermes-Customers já está com ambiente local
   validado nesta sessão; Hermes-Movies e Hermes-OrderFlow-Analytics-Platform
   precisariam do mesmo check de pré-condição antes)
2. **Qual endpoint/fluxo é o mais representativo** daquele projeto para medir
   primeiro?
3. Onde este arquivo deve viver de forma permanente — copiado para dentro de
   cada repo (`docs/testing/`, como aqui), ou centralizado numa pasta
   compartilhada (`QA/`) fora dos repositórios individuais?
