FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o mundo-invest ./customers/cmd/main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/mundo-invest .
EXPOSE 8080
CMD ["./mundo-invest"]
