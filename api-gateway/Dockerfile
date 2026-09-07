FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o api-gateway ./cmd/main.go

FROM alpine:3.19
RUN addgroup -S -g 10001 app && adduser -S -u 10001 -G app app
WORKDIR /app
COPY --from=builder /app/api-gateway .
RUN chown -R app:app /app
USER app
ENV GIN_MODE=release
EXPOSE 8000
CMD ["./api-gateway"]
