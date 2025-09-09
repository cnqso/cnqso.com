FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

RUN go build -o server .

FROM alpine:latest

RUN apk --no-cache add ca-certificates curl sqlite

RUN addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup

WORKDIR /app

RUN mkdir -p /app/uploads && \
    chown -R appuser:appgroup /app

COPY --from=builder /app .

RUN chown -R appuser:appgroup .

USER appuser

EXPOSE 1738

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:1738/health || exit 1

CMD ["./server"]
