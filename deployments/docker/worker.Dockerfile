# ===== Stage 1: 后端构建 =====
FROM golang:1.25-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    protobuf-compiler \
    && rm -rf /var/lib/apt/lists/* \
    && go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 生成 protobuf 代码
RUN PATH="$PATH:$(go env GOPATH)/bin" protoc --go_out=pkg/grpc/pb --go_opt=paths=source_relative \
    --go-grpc_out=pkg/grpc/pb --go-grpc_opt=paths=source_relative \
    --proto_path=pkg/grpc/proto \
    pkg/grpc/proto/*.proto

# 构建 Worker
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/bin/worker cmd/worker/main.go

# ===== Stage 2: 运行环境 (Debian + Python) =====
FROM python:3.12-slim-bookworm

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    bash \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/bin/worker /app/worker

RUN mkdir -p /app/logs

CMD ["/app/worker"]
