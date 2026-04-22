# ===== Stage 1: 前端构建 =====
FROM node:20-slim AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# ===== Stage 2: 后端构建 =====
FROM golang:1.25-bookworm AS backend-builder

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

# 从前端阶段复制构建产物
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# 构建 Manager
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/bin/manager cmd/manager/main.go

# ===== Stage 3: 运行环境 =====
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=backend-builder /app/bin/manager /app/manager
COPY --from=backend-builder /app/frontend/dist /app/frontend/dist

RUN mkdir -p /app/logs

EXPOSE 8080 9090 8081

CMD ["/app/manager"]
