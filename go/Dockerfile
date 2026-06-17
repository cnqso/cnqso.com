# syntax=docker/dockerfile:1.7

FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . ./

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -o server .

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl sqlite tzdata nodejs npm

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

WORKDIR /app

RUN mkdir -p /app/uploads && \
    chown -R appuser:appgroup /app

COPY --from=builder --chown=appuser:appgroup /app .

USER appuser

EXPOSE 1738

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:1738/health || exit 1

CMD ["./server"]
