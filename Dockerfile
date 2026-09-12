# The module depends on goplatform through "replace ../platforme", so the
# build context is the PARENT directory that holds both checkouts:
#
#   docker build -f expenses/Dockerfile -t expenses ..
#
# Not exercised on this machine yet; treat as a starting point.

FROM node:20-alpine AS web
WORKDIR /src/expenses/web
COPY expenses/web/package.json expenses/web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY expenses/web ./
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY platforme ./platforme
COPY expenses ./expenses
COPY --from=web /src/expenses/web/dist ./expenses/web/dist
WORKDIR /src/expenses
RUN go mod download && CGO_ENABLED=0 go build -o /bin/expenses ./cmd

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /bin/expenses /bin/expenses
COPY expenses/config ./config
EXPOSE 8080
ENTRYPOINT ["/bin/expenses"]
