#!/bin/bash

# ================= 配置区域 =================
GO_VERSION="1.25.0"
GO_PACKAGE="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_PACKAGE}"

# 国内镜像源配置 (七牛云)
GOPROXY_URL="https://goproxy.cn,direct"
# 国内校验和数据库 (解决 sum 下载超时)
GOSUMDB_URL="sum.golang.org+https://sum.golang.google.cn"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
WHITE='\033[1;37m'
NC='\033[0m'

# 打印函数
log_info() { printf "${GREEN}>>> %b${NC}\n" "$1"; }
log_warn() { printf "${YELLOW}>>> %b${NC}\n" "$1"; }
log_err() { printf "${RED}>>> %b${NC}\n" "$1"; }
# ===========================================

log_info "开始安装 Go ${GO_VERSION}..."

# 1. 检查并下载
if [ ! -f "$GO_PACKAGE" ]; then
    log_warn "当前目录未找到安装包，正在下载..."
    if ! wget -q --show-progress "$GO_URL"; then
        log_err "下载失败，请检查网络或手动下载后重试。"
        exit 1
    fi
else
    log_info "发现本地安装包: $GO_PACKAGE"
fi

# 2. 检查已安装的 Go 版本
if command -v go > /dev/null 2>&1; then
    INSTALLED_VERSION=$(go version | sed -n 's/.*go\([0-9][0-9]*\.[0-9][0-9]*\(\.[0-9][0-9]*\)\).*/\1/p')
    if [ -z "$INSTALLED_VERSION" ]; then
        log_err "检测到 go 命令存在，但无法解析版本号，请检查环境。"
        exit 1
    fi
    if [ "$INSTALLED_VERSION" = "$GO_VERSION" ]; then
        log_info "已安装 Go $INSTALLED_VERSION，与目标版本一致，跳过安装。"
        exit 0
    else
        log_err "检测到已安装 Go $INSTALLED_VERSION，与目标版本 $GO_VERSION 不一致。"
        log_err "请先手动卸载现有版本后重新运行本脚本。"
        exit 1
    fi
fi

# 3. 解压安装
log_info "正在解压到 /usr/local..."
if sudo tar -C /usr/local -xzf "$GO_PACKAGE"; then
    log_info "解压成功。"
else
    log_err "解压失败。"
    exit 1
fi

# 4. 配置环境变量
log_warn "正在配置环境变量..."
if ! grep -q "GOROOT=/usr/local/go" ~/.bashrc; then
    cat << EOF >> ~/.bashrc

# Go Environment (Added by script)
export GOROOT=/usr/local/go
export PATH=\$PATH:\$GOROOT/bin
EOF
    log_info "环境变量已添加到 ~/.bashrc"
else
    log_warn "环境变量已存在，跳过。"
fi

# 5. 配置 Go 代理 (新增功能)
log_warn "正在配置 Go 模块镜像源..."
# 注意：这里直接写入配置文件，避免依赖 go env -w 命令（因为此时 PATH 可能还没刷新）
if ! grep -q "GOPROXY" ~/.bashrc; then
    cat << EOF >> ~/.bashrc

# Go Proxy Settings
export GOPROXY=${GOPROXY_URL}
export GOSUMDB=${GOSUMDB_URL}
EOF
    log_info "镜像源配置已添加到 ~/.bashrc"
else
    log_warn "镜像源配置已存在，跳过。"
fi

# 6. 刷新当前 shell 的 PATH（使刚配置的 Go 立即可用）
export PATH=$PATH:/usr/local/go/bin

# 7. 安装 Go 依赖
log_warn "正在下载 Go 模块依赖 (go mod tidy)..."
if go mod tidy; then
    log_info "Go 依赖安装完成。"
else
    log_err "Go 依赖安装失败，请检查网络或手动执行 go mod tidy。"
fi

# 8. 安装前端依赖
if [ -d "frontend" ] && [ -f "frontend/package.json" ]; then
    log_warn "正在安装 frontend 依赖 (npm install)..."
    if (cd frontend && npm install); then
        log_info "前端依赖安装完成。"
    else
        log_err "前端依赖安装失败，请检查 Node.js 环境后手动执行 cd frontend && npm install。"
    fi
else
    log_warn "未找到 frontend 目录或 package.json，跳过前端依赖安装。"
fi

# 9. 完成提示
printf "\n"
printf "${GREEN}========================================${NC}\n"
log_info "安装与配置全部完成！"
log_warn "请执行以下命令使配置立即生效："
printf "       ${WHITE}source ~/.bashrc${NC}\n"
log_warn "生效后，请运行 'go version' 验证。"
printf "${GREEN}========================================${NC}\n"
