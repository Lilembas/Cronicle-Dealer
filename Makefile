.PHONY: all proto build clean run-manager run-worker docker test

# 默认目标
all: proto build

# 生成 Protobuf 代码
proto:
	@echo "生成 Protobuf 代码..."
	@export PATH=$$(go env GOPATH)/bin:$$PATH; \
	protoc --go_out=pkg/grpc/pb --go_opt=paths=source_relative \
		--go-grpc_out=pkg/grpc/pb --go-grpc_opt=paths=source_relative \
		--proto_path=pkg/grpc/proto \
		pkg/grpc/proto/*.proto
	@echo "✅ Protobuf 代码生成完成"

# 构建二进制文件
build: build-manager build-worker

build-manager:
	@echo "构建 Manager..."
	go build -o bin/manager cmd/manager/main.go
	@echo "✅ Manager 构建完成"

build-worker:
	@echo "构建 Worker..."
	go build -o bin/worker cmd/worker/main.go
	@echo "✅ Worker 构建完成"

# 运行服务
run-manager:
	@echo "启动 Manager..."
	go run cmd/manager/main.go

run-worker:
	@echo "启动 Worker..."
	go run cmd/worker/main.go

# 安装依赖
deps:
	@echo "安装 Go 依赖..."
	go mod download
	go mod tidy
	@echo "✅ 依赖安装完成"

# 安装 Protobuf 编译器插件
install-proto-tools:
	@echo "安装 Protobuf 工具..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✅ Protobuf 工具安装完成"

# Docker 构建
docker: docker-manager docker-worker

docker-manager:
	@echo "构建 Manager Docker 镜像..."
	docker build -f deployments/docker/manager.Dockerfile -t cronicle-manager:latest .
	@echo "✅ Manager 镜像构建完成"

docker-worker:
	@echo "构建 Worker Docker 镜像..."
	docker build -f deployments/docker/worker.Dockerfile -t cronicle-worker:latest .
	@echo "✅ Worker 镜像构建完成"

# 运行 Docker Compose
docker-up:
	docker-compose -f deployments/docker-compose.yml up -d

docker-down:
	docker-compose -f deployments/docker-compose.yml down

docker-logs:
	docker-compose -f deployments/docker-compose.yml logs -f

# 测试
test:
	@echo "运行测试..."
	go test -v ./...

test-coverage:
	@echo "运行测试并生成覆盖率报告..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ 覆盖率报告已生成: coverage.html"

integration-test:
	@echo "运行集成测试..."
	@cd test && go run integration_test.go

# 清理历史记录
clean-events:
	@echo "清理历史记录..."
	@./scripts/clear_events.sh

clean-events-selective:
	@echo "选择性清理历史记录..."
	@./scripts/clear_events_selective.sh

# 查看历史记录统计
stats-events:
	@echo "📊 历史记录统计:"
	@echo ""
	@sqlite3 cronicle.db "SELECT '总记录数: ' || COUNT(*) FROM events;"
	@sqlite3 cronicle.db "SELECT '  成功: ' || COUNT(*) FROM events WHERE status='success';"
	@sqlite3 cronicle.db "SELECT '  失败: ' || COUNT(*) FROM events WHERE status='failed';"
	@sqlite3 cronicle.db "SELECT '  运行中: ' || COUNT(*) FROM events WHERE status='running';"

# 清理
clean:
	@echo "清理构建文件..."
	rm -rf bin/
	rm -rf logs/
	rm -f coverage.out coverage.html
	@echo "✅ 清理完成"

clean-all: clean clean-events
	@echo "✅ 清理构建文件和历史记录完成"

# 格式化代码
fmt:
	@echo "格式化代码..."
	go fmt ./...
	@echo "✅ 代码格式化完成"

# 代码检查
lint:
	@echo "运行代码检查..."
	golangci-lint run
	@echo "✅ 代码检查完成"

# 开发环境
dev: deps proto
	@echo "✅ 开发环境准备完成"

# 帮助信息
help:
	@echo "可用命令："
	@echo "  make proto                - 生成 Protobuf 代码"
	@echo "  make build                - 构建所有二进制文件"
	@echo "  make run-manager           - 运行 Manager"
	@echo "  make run-worker           - 运行 Worker"
	@echo "  make docker               - 构建 Docker 镜像"
	@echo "  make docker-up            - 启动 Docker Compose"
	@echo "  make test                 - 运行测试"
	@echo "  make integration-test    - 运行集成测试"
	@echo "  make clean                - 清理构建文件"
	@echo "  make clean-events         - 清理所有历史记录"
	@echo "  make clean-events-selective - 选择性清理历史记录"
	@echo "  make stats-events         - 查看历史记录统计"
	@echo "  make clean-all            - 清理构建文件和历史记录"
	@echo "  make fmt                  - 格式化代码"
	@echo "  make lint                 - 代码检查"
	@echo "  make dev                  - 准备开发环境"
