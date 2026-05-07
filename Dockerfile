# ── Build stage ───────────────────────────────────────────
FROM golang:1.24-bookworm AS builder

RUN apt-get update && apt-get install -y \
    protobuf-compiler libsqlite3-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /build

# Copiar dependencias primero para cache
COPY go.mod go.sum ./
RUN go mod download

# Copiar codigo fuente
COPY . .

RUN CGO_ENABLED=1 go build \
    -ldflags "-s -w" \
    -o bin/snapah-server ./cmd/server && \
    CGO_ENABLED=1 go build \
    -ldflags "-s -w" \
    -o bin/snapah-agent ./cmd/agent && \
    CGO_ENABLED=1 go build \
    -ldflags "-s -w" \
    -o bin/snapah ./cmd/cli

# ── Runtime stage ─────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates libsqlite3-0 btrfs-progs curl && \
    rm -rf /var/lib/apt/lists/* && \
    useradd -r -s /bin/false snapah

WORKDIR /opt/snapah

COPY --from=builder /build/bin/snapah-server  ./bin/
COPY --from=builder /build/bin/snapah-agent   ./bin/
COPY --from=builder /build/bin/snapah         ./bin/
COPY --from=builder /build/web                ./web/
COPY --from=builder /build/config.yaml        ./

RUN mkdir -p data && chown -R snapah:snapah /opt/snapah

USER snapah

EXPOSE 8082 9091 9093

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8082/health || exit 1

ENTRYPOINT ["./bin/snapah-server"]
