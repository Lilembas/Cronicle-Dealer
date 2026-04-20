FROM golang:1.22-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git make protobuf-dev

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建 Manager
RUN go build -o /app/bin/manager cmd/manager/main.go

# 运行镜像
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 从构建镜像复制二进制文件
COPY --from=builder /app/bin/manager /app/manager

# 创建日志目录
RUN mkdir -p /app/logs

# 暴露端口
EXPOSE 8080 9090

# 运行 Manager
CMD ["/app/manager"]
