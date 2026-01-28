# 测试脚本故障排除

## 问题: "❌ Go 未安装或不在 PATH 中"

### 症状
```bash
$ ./test/run_worker_test.sh
🔍 检查依赖...
❌ Go 未安装或不在 PATH 中
```

但运行 `go version` 显示 Go 已安装：
```bash
$ go version
go version go1.25.6 linux/amd64
```

---

## 解决方案

### 方案 1: 临时添加 Go 到 PATH（推荐）

在运行测试脚本前设置 PATH：

```bash
# 临时设置（仅当前会话有效）
export PATH=$PATH:/usr/local/go/bin

# 然后运行测试
./test/run_worker_test.sh
```

---

### 方案 2: 永久添加 Go 到 PATH

编辑你的 shell 配置文件：

#### 对于 Bash
```bash
# 编辑 ~/.bashrc 或 ~/.bash_profile
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

#### 对于 Zsh
```bash
# 编辑 ~/.zshrc
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.zshrc
source ~/.zshrc
```

---

### 方案 3: 使用测试脚本的自动检测

测试脚本现在会自动检测以下 Go 安装路径：
- `go` (PATH 中)
- `/usr/local/go/bin/go`
- `/usr/bin/go`
- `$HOME/go/bin/go`

如果 Go 在上述路径中，脚本会自动使用它。

**输出示例：**
```bash
$ ./test/run_worker_test.sh
🔍 检查依赖...
✅ 依赖检查通过 (Go: /usr/local/go/bin/go)
```

---

### 方案 4: 创建软链接

如果你没有权限修改 PATH，可以创建软链接：

```bash
# 需要 sudo 权限
sudo ln -s /usr/local/go/bin/go /usr/local/bin/go

# 验证
which go
# 应该输出: /usr/local/bin/go
```

---

## 其他常见问题

### 1. 配置文件不存在
```bash
❌ 配置文件 ../config.yaml 不存在
```

**解决方法：**
```bash
cp config.example.yaml config.yaml
```

---

### 2. Redis 连接失败
```bash
❌ Redis 连接失败: dial tcp: connection refused
```

**解决方法：**
```bash
# 启动本地 Redis
redis-server

# 或使用 Docker
docker-compose -f deployments/docker-compose.yml up -d redis
```

---

### 3. 权限不足
```bash
bash: ./test/run_worker_test.sh: Permission denied
```

**解决方法：**
```bash
chmod +x test/run_worker_test.sh
chmod +x test/run_e2e_test.sh
chmod +x test/run_integration_test.sh
```

---

### 4. nc 命令不存在
```bash
nc: command not found
```

**解决方法：**
```bash
# CentOS/RHEL
sudo yum install nc

# Ubuntu/Debian
sudo apt install netcat
```

---

## 验证 Go 安装

运行以下命令验证 Go 是否正确安装：

```bash
# 检查 Go 版本
go version

# 检查 Go 位置
which go

# 检查 GOPATH 和 GOROOT
go env GOPATH
go env GOROOT

# 测试 Go 编译
go version # 应该输出版本信息
```

---

## 完整测试流程

```bash
# 1. 设置环境
export PATH=$PATH:/usr/local/go/bin

# 2. 准备配置
cp config.example.yaml config.yaml

# 3. 确保 Redis 运行
redis-server --daemonize yes

# 4. 运行测试
./test/run_worker_test.sh
```

---

## 仍然有问题？

如果上述方案都无法解决问题，请：

1. 检查 Go 是否真的安装：
   ```bash
   ls -la /usr/local/go/bin/go
   ```

2. 手动指定 Go 路径编辑脚本：
   ```bash
   # 在测试脚本中设置
   GO_BIN="/usr/local/go/bin/go"
   ```

3. 查看 Go 官方安装文档：https://go.dev/doc/install

4. 在项目 GitHub Issues 中报告问题
