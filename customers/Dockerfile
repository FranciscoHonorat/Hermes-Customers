FROM golang:1.25-alpine AS builder

WORKDIR /app
# customers é um módulo Go próprio (github.com/.../mundo-invest/customers);
# copiamos só o go.mod/go.sum dele primeiro para cachear o download de
# dependências. O build final roda em modo workspace (go.work, copiado a
# seguir) porque customers importa o módulo local "shared".
COPY customers/go.mod customers/go.sum ./customers/
RUN cd customers && go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o mundo-invest ./customers/cmd/main.go

FROM alpine:3.19
RUN addgroup -S -g 10001 app && adduser -S -u 10001 -G app app
WORKDIR /app
COPY --from=builder /app/mundo-invest .
RUN mkdir -p /data && chown -R app:app /app /data
USER app
ENV GIN_MODE=release
EXPOSE 8080
CMD ["./mundo-invest"]
