#!/bin/bash

# ================= 配置区域 =================
# Go 配置
GO_VERSION="1.25.0"
GO_PACKAGE="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_PACKAGE}"
GOPROXY_URL="https://goproxy.cn,direct"
GOSUMDB_URL="sum.golang.org+https://sum.golang.google.cn"

# Node.js 配置
NODE_VERSION="20.12.2"
NODE_PACKAGE="node-v${NODE_VERSION}-linux-x64.tar.xz"
NODE_URL="https://nodejs.org/dist/v${NODE_VERSION}/${NODE_PACKAGE}"
NPM_REGISTRY="https://registry.npmmirror.com"

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
INSTALL_GO=true
if command -v go > /dev/null 2>&1; then
    INSTALLED_VERSION=$(go version | sed -n 's/.*go\([0-9][0-9]*\.[0-9][0-9]*\(\.[0-9][0-9]*\)\).*/\1/p')
    if [ "$INSTALLED_VERSION" = "$GO_VERSION" ]; then
        log_info "已安装 Go $INSTALLED_VERSION，与目标版本一致，跳过安装。"
        INSTALL_GO=false
    else
        log_warn "检测到已安装 Go $INSTALLED_VERSION，与目标版本 $GO_VERSION 不一致。"
        log_warn "正在准备更新/覆盖安装..."
    fi
fi

if [ "$INSTALL_GO" = true ]; then
    # 3. 解压安装 Go
    log_info "正在安装 Go ${GO_VERSION} 到 /usr/local..."
    if sudo tar -C /usr/local -xzf "$GO_PACKAGE"; then
        log_info "Go 解压成功。"
    else
        log_err "Go 解压失败。"
        exit 1
    fi

    # 4. 配置 Go 环境变量
    log_warn "正在配置 Go 环境变量..."
    if ! grep -q "GOROOT=/usr/local/go" ~/.bashrc; then
        cat << EOF >> ~/.bashrc

# Go Environment (Added by script)
export GOROOT=/usr/local/go
export PATH=\$PATH:\$GOROOT/bin
EOF
        log_info "Go 环境变量已添加到 ~/.bashrc"
    fi

    # 5. 配置 Go 代理
    log_warn "正在配置 Go 模块镜像源..."
    if ! grep -q "GOPROXY" ~/.bashrc; then
        cat << EOF >> ~/.bashrc
export GOPROXY=${GOPROXY_URL}
export GOSUMDB=${GOSUMDB_URL}
EOF
        log_info "Go 镜像源配置已添加到 ~/.bashrc"
    fi
fi

# 6. 安装 Node.js
log_info "开始检查 Node.js..."
INSTALL_NODE=true
if command -v node > /dev/null 2>&1; then
    INSTALLED_NODE_VER=$(node -v | sed 's/v//')
    log_info "已安装 Node.js $INSTALLED_NODE_VER"
    # 如果版本大于等于 18，通常可以跳过
    if [ "$(printf '%s\n' "$INSTALLED_NODE_VER" "$NODE_VERSION" | sort -V | head -n1)" = "$NODE_VERSION" ] || [ "${INSTALLED_NODE_VER%%.*}" -ge 18 ]; then
        log_info "当前 Node.js 版本满足要求，跳过安装。"
        INSTALL_NODE=false
    else
        log_warn "当前 Node.js 版本较旧，建议更新。"
    fi
fi

if [ "$INSTALL_NODE" = true ]; then
    if [ ! -f "$NODE_PACKAGE" ]; then
        log_warn "正在下载 Node.js $NODE_VERSION..."
        if ! wget -q --show-progress "$NODE_URL"; then
            log_err "Node.js 下载失败。"
            exit 1
        fi
    fi

    log_info "正在安装 Node.js ${NODE_VERSION} 到 /usr/local..."
    # Node.js tarball 通常包含一个顶层目录 node-v...-linux-x64
    # 我们将其内容提取到 /usr/local
    sudo mkdir -p /usr/local/lib/node_modules
    if sudo tar -xJf "$NODE_PACKAGE" --strip-components=1 -C /usr/local; then
        log_info "Node.js 安装成功。"
    else
        log_err "Node.js 安装失败。"
        exit 1
    fi

    # 配置 NPM 镜像
    log_warn "正在配置 NPM 镜像源..."
    /usr/local/bin/npm config set registry ${NPM_REGISTRY}
    log_info "NPM 镜像源已设置为: ${NPM_REGISTRY}"
fi

# 7. 刷新当前 shell 的 PATH
export PATH=$PATH:/usr/local/go/bin:/usr/local/bin

# 8. 安装 Go 依赖
log_warn "正在下载 Go 模块依赖 (go mod tidy)..."
if command -v go > /dev/null 2>&1; then
    if go mod tidy; then
        log_info "Go 依赖安装完成。"
    else
        log_err "Go 依赖安装失败，请检查网络或手动执行 go mod tidy。"
    fi
else
    log_err "未找到 go 命令，跳过依赖安装。"
fi

# 9. 安装前端依赖
if [ -d "frontend" ] && [ -f "frontend/package.json" ]; then
    log_warn "正在安装 frontend 依赖 (npm install)..."
    if command -v npm > /dev/null 2>&1; then
        if (cd frontend && npm install); then
            log_info "前端依赖安装完成。"
        else
            log_err "前端依赖安装失败，请检查 Node.js 环境后手动执行 cd frontend && npm install。"
        fi
    else
        log_err "未找到 npm 命令，跳过前端依赖安装。"
    fi
else
    log_warn "未找到 frontend 目录或 package.json，跳过前端依赖安装。"
fi

# 10. 完成提示
printf "\n"
printf "${GREEN}========================================${NC}\n"
log_info "安装与配置全部完成！"
log_warn "请执行以下命令使配置立即生效："
printf "       ${WHITE}source ~/.bashrc${NC}\n"
log_warn "生效后，请运行 'go version' 和 'node -v' 验证。"
printf "${GREEN}========================================${NC}\n"
