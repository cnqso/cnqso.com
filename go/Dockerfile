# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS typescript
WORKDIR /src
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --ignore-scripts
COPY tsconfig.json ./
COPY ts ./ts
RUN npm run build

FROM node:22-alpine AS spirals
WORKDIR /src/submodules/spirals
COPY submodules/spirals/package.json submodules/spirals/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --legacy-peer-deps
COPY submodules/spirals ./
RUN mkdir -p /src/go/static/react/spirals && PUBLIC_URL=/spirals npm run build

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY go/go.mod go/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY go ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go test ./... && go build -o /out/server . && go build -o /out/library-password ./cmd/library-password

FROM alpine:3.22
RUN apk --no-cache add ca-certificates curl tzdata && \
    addgroup -g 1000 -S appgroup && \
    adduser -u 1000 -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /out/ ./
COPY --chown=appuser:appgroup go/templates ./templates
COPY --chown=appuser:appgroup go/blog-posts ./blog-posts
COPY --chown=appuser:appgroup go/static ./static
COPY --from=typescript --chown=appuser:appgroup /src/go/static/js ./static/js
COPY --from=spirals --chown=appuser:appgroup /src/go/static/react/spirals ./static/react/spirals
RUN mkdir -p db uploads files/public static/petrarchive && chown -R appuser:appgroup db uploads files static/petrarchive
USER appuser
ENV CNQSO_COMPILE_TYPESCRIPT=false
EXPOSE 1738
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:1738/health || exit 1
CMD ["./server"]
