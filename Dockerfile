# Dockerfile — Producción (multi-stage build)
# Resultado: imagen Alpine mínima ~20MB

# ── Stage 1: Build ──────────────────────────────
FROM golang:1.26.1-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compilar binario estático (sin dependencias externas)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o nomad-api \
    ./cmd/server

# ── Stage 2: Runtime ────────────────────────────
FROM alpine:3.19

# Certificados TLS (necesario para llamadas HTTPS a S3, etc.)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/nomad-api .

EXPOSE 8080

CMD ["./nomad-api"]
