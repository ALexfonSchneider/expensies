# Builds the expenses app: React frontend, then the Go binary with the
# frontend embedded, into a minimal runtime image.
#
# The module depends on goplatform through "replace ../platforme", so the
# build context is the PARENT directory that holds both checkouts:
#
#   docker build -f expenses/Dockerfile -t expenses ..
#
# docker compose in expenses/ already points the context there.

# ---- frontend -------------------------------------------------------------
FROM node:20-alpine AS web
WORKDIR /src/web
COPY expenses/web/package.json expenses/web/package-lock.json ./
# npm ci runs the postinstall hook that writes node_modules/go.mod; that
# file matters only for the Go toolchain on a developer machine.
RUN npm ci --no-audit --no-fund
COPY expenses/web ./
RUN npm run build

# ---- backend --------------------------------------------------------------
FROM golang:1.25-alpine AS builder
WORKDIR /src
# Dependency files first so the module cache survives source edits.
COPY platforme/go.mod platforme/go.sum ./platforme/
COPY expenses/go.mod expenses/go.sum ./expenses/
WORKDIR /src/expenses
RUN --mount=type=cache,target=/go/pkg/mod go mod download
WORKDIR /src
COPY platforme ./platforme
COPY expenses ./expenses
COPY --from=web /src/web/dist ./expenses/web/dist
WORKDIR /src/expenses
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/expenses ./cmd

# ---- runtime --------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/expenses /app/expenses
COPY expenses/config ./config
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz/ready >/dev/null || exit 1
ENTRYPOINT ["/app/expenses"]
