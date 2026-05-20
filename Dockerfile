# syntax=docker/dockerfile:1

ARG NODE_VERSION=20
ARG GO_VERSION=1.25

FROM node:${NODE_VERSION}-slim AS frontend-builder

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ ./
RUN npm run build

FROM golang:${GO_VERSION}-bookworm AS backend-builder

WORKDIR /src

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    libc6-dev \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /src/frontend/dist ./frontend/dist

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/manager ./cmd/manager

RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/worker ./cmd/worker

FROM debian:bookworm-slim AS manager

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="cronicle-manager" \
    org.opencontainers.image.version="${VERSION}" \
    org.opencontainers.image.revision="${COMMIT}" \
    org.opencontainers.image.created="${BUILD_DATE}"

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /app/data /app/logs /app/frontend

COPY --from=backend-builder /out/manager /app/manager
COPY --from=backend-builder /src/frontend/dist /app/frontend/dist

ENV CRONICLE_MANAGER_DATABASE_PATH=/app/data/cronicle.db \
    CRONICLE_LOGGING_LOG_DIR=/app/logs

EXPOSE 8080 9090

CMD ["/app/manager"]

FROM python:3.12-slim-bookworm AS worker

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="cronicle-worker" \
    org.opencontainers.image.version="${VERSION}" \
    org.opencontainers.image.revision="${COMMIT}" \
    org.opencontainers.image.created="${BUILD_DATE}"

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /app/data /app/logs

COPY --from=backend-builder /out/worker /app/worker

ENV CRONICLE_WORKER_NODE_ID_FILE=/app/data/worker_nodes.json \
    CRONICLE_LOGGING_LOG_DIR=/app/logs

EXPOSE 50051

CMD ["/app/worker"]
