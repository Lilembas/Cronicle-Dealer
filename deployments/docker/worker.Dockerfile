FROM golang:1.22-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git make protobuf-dev

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建 Worker
RUN go build -o /app/bin/worker cmd/worker/main.go

# 运行镜像
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖（Worker 需要能执行 shell 命令）
RUN apk add --no-cache ca-certificates tzdata bash curl

# 从构建镜像复制二进制文件
COPY --from=builder /app/bin/worker /app/worker

# 创建日志目录
RUN mkdir -p /app/logs

# 运行 Worker
CMD ["/app/worker"]
