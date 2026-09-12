# Builds the expenses app: React frontend, then the Go binary with the
# frontend embedded, into a minimal runtime image.
#
#   docker build -t expenses .
#
# goplatform is a versioned dependency fetched from the module proxy, so
# the build context is this directory alone.

# ---- frontend -------------------------------------------------------------
FROM node:20-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
# npm ci runs the postinstall hook that writes node_modules/go.mod; that
# file matters only for the Go toolchain on a developer machine.
RUN npm ci --no-audit --no-fund
COPY web ./
RUN npm run build

# ---- backend --------------------------------------------------------------
FROM golang:1.25-alpine AS builder
# git lets the toolchain fall back to fetching a module straight from its
# repository when the public proxy has not indexed a fresh tag yet.
RUN apk add --no-cache git
WORKDIR /src
# Dependency files first so the module cache survives source edits.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/expenses ./cmd

# ---- runtime --------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/expenses /app/expenses
COPY config ./config
USER app
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz/ready >/dev/null || exit 1
ENTRYPOINT ["/app/expenses"]
