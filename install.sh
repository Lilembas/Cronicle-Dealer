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
NC='\033[0m'
# ===========================================

echo -e "${GREEN}>>> 开始安装 Go ${GO_VERSION}...${NC}"

# 1. 检查并下载
if [ ! -f "$GO_PACKAGE" ]; then
    echo -e "${YELLOW}>>> 当前目录未找到安装包，正在下载...${NC}"
    if ! wget -q --show-progress "$GO_URL"; then
        echo -e "${RED}>>> 下载失败，请检查网络或手动下载后重试。${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}>>> 发现本地安装包: $GO_PACKAGE${NC}"
fi

# 2. 检查已安装的 Go 版本
if go version &> /dev/null; then
    INSTALLED_VERSION=$(go version | sed -n 's/.*go\([0-9][0-9]*\.[0-9][0-9]*\(\.[0-9][0-9]*\)\).*/\1/p')
    if [ -z "$INSTALLED_VERSION" ]; then
        echo -e "${RED}>>> 检测到 go 命令存在，但无法解析版本号，请检查环境。${NC}"
        exit 1
    fi
    if [ "$INSTALLED_VERSION" = "$GO_VERSION" ]; then
        echo -e "${GREEN}>>> 已安装 Go $INSTALLED_VERSION，与目标版本一致，跳过安装。${NC}"
        exit 0
    else
        echo -e "${RED}>>> 检测到已安装 Go $INSTALLED_VERSION，与目标版本 $GO_VERSION 不一致。${NC}"
        echo -e "${RED}>>> 请先手动卸载现有版本后重新运行本脚本。${NC}"
        exit 1
    fi
fi

# 3. 解压安装
echo -e "${GREEN}>>> 正在解压到 /usr/local...${NC}"
if sudo tar -C /usr/local -xzf "$GO_PACKAGE"; then
    echo -e "${GREEN}>>> 解压成功。${NC}"
else
    echo -e "${RED}>>> 解压失败。${NC}"
    exit 1
fi

# 4. 配置环境变量
echo -e "${YELLOW}>>> 正在配置环境变量...${NC}"
if ! grep -q "GOROOT=/usr/local/go" ~/.bashrc; then
    cat << EOF >> ~/.bashrc

# Go Environment (Added by script)
export GOROOT=/usr/local/go
export PATH=\$PATH:\$GOROOT/bin
EOF
    echo -e "${GREEN}>>> 环境变量已添加到 ~/.bashrc${NC}"
else
    echo -e "${YELLOW}>>> 环境变量已存在，跳过。${NC}"
fi

# 5. 配置 Go 代理 (新增功能)
echo -e "${YELLOW}>>> 正在配置 Go 模块镜像源...${NC}"
# 注意：这里直接写入配置文件，避免依赖 go env -w 命令（因为此时 PATH 可能还没刷新）
if ! grep -q "GOPROXY" ~/.bashrc; then
    cat << EOF >> ~/.bashrc

# Go Proxy Settings
export GOPROXY=${GOPROXY_URL}
export GOSUMDB=${GOSUMDB_URL}
EOF
    echo -e "${GREEN}>>> 镜像源配置已添加到 ~/.bashrc${NC}"
else
    echo -e "${YELLOW}>>> 镜像源配置已存在，跳过。${NC}"
fi

# 6. 刷新当前 shell 的 PATH（使刚配置的 Go 立即可用）
export PATH=$PATH:/usr/local/go/bin

# 7. 安装 Go 依赖
echo -e "${YELLOW}>>> 正在下载 Go 模块依赖 (go mod tidy)...${NC}"
if go mod tidy; then
    echo -e "${GREEN}>>> Go 依赖安装完成。${NC}"
else
    echo -e "${RED}>>> Go 依赖安装失败，请检查网络或手动执行 go mod tidy。${NC}"
fi

# 8. 安装前端依赖
if [ -d "frontend" ] && [ -f "frontend/package.json" ]; then
    echo -e "${YELLOW}>>> 正在安装前端依赖 (npm install)...${NC}"
    if (cd frontend && npm install); then
        echo -e "${GREEN}>>> 前端依赖安装完成。${NC}"
    else
        echo -e "${RED}>>> 前端依赖安装失败，请检查 Node.js 环境后手动执行 cd frontend && npm install。${NC}"
    fi
else
    echo -e "${YELLOW}>>> 未找到 frontend 目录或 package.json，跳过前端依赖安装。${NC}"
fi

# 9. 完成提示
echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}>>> 安装与配置全部完成！${NC}"
echo -e "${YELLOW}>>> 请执行以下命令使配置立即生效：${NC}"
echo -e "       ${WHITE}source ~/.bashrc${NC}"
echo -e "${YELLOW}>>> 生效后，请运行 'go version' 验证。${NC}"
echo -e "${GREEN}========================================${NC}"