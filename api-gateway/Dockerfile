FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o api-gateway ./cmd/main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/api-gateway .
EXPOSE 8000
CMD ["./api-gateway"]
