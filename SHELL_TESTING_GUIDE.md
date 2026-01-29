# Shell 执行功能测试指南

## 🚀 快速测试

### 1. 启动服务

**终端 1 - 启动 Master**:
```bash
cd /codespace/developers/linnan/claudeProjects/cronicle-next
go run cmd/master/main.go
```

**终端 2 - 启动 Worker**:
```bash
cd /codespace/developers/linnan/claudeProjects/cronicle-next
go run cmd/worker/main.go
```

**终端 3 - 启动前端**:
```bash
cd /codespace/developers/linnan/claudeProjects/cronicle-next/frontend
npm run dev
```

### 2. 访问页面

浏览器打开：`http://localhost:5173`

### 3. 导航到 Shell 执行页面

点击侧边栏的 **"Shell 执行"** 菜单项，或直接访问：
```
http://localhost:5173/shell
```

## ✅ 测试步骤

### 测试 1: 快速命令

1. 点击 **"当前目录"** 按钮
2. 点击 **"执行"** 按钮
3. 观察输出区域显示命令结果
4. 查看退出码标签（应该显示绿色 "退出码: 0"）

### 测试 2: 自定义命令

1. 在输入框中输入：`echo "Hello, Cronicle!"`
2. 点击 **"执行"** 按钮
3. 查看实时输出
4. 等待命令完成

### 测试 3: 长时间运行命令

1. 输入：`sleep 5 && echo "完成"`
2. 执行命令
3. 观察轮询过程中的实时输出更新
4. 验证 5 秒后显示"完成"

### 测试 4: 错误命令

1. 输入：`ls /nonexistent`
2. 执行命令
3. 验证显示错误信息
4. 查看退出码（非 0，红色标签）

### 测试 5: 系统信息命令

1. 点击 **"系统信息"** 按钮
2. 执行命令
3. 查看系统信息输出

## 🔍 验证检查点

- [ ] 页面正常加载，无控制台错误
- [ ] 命令输入框可以正常输入
- [ ] 快速命令按钮可以填入命令
- [ ] 执行按钮在输入为空时禁用
- [ ] 执行中显示加载动画
- [ ] 日志输出区域实时更新（每 500ms）
- [ ] 命令完成后显示正确的退出码
- [ ] 清空按钮可以清空输出
- [ ] 执行中输入框和快速命令被禁用

## 📊 API 测试

### 使用 curl 测试

**1. 执行命令**:
```bash
curl -X POST http://localhost:8080/api/v1/shell/execute \
  -H "Content-Type: application/json" \
  -d '{"command": "pwd"}'
```

**预期响应**:
```json
{
  "event_id": "shell_event_1234567890",
  "job_id": "shell_adhoc_1234567890",
  "command": "pwd",
  "status": "queued",
  "message": "任务已提交，正在执行中",
  "node_id": "node-abc123"
}
```

**2. 获取日志**:
```bash
curl http://localhost:8080/api/v1/shell/logs/shell_event_1234567890
```

**预期响应**:
```json
{
  "event_id": "shell_event_1234567890",
  "logs": "/codespace/developers/linnan/claudeProjects/cronicle-next",
  "complete": true,
  "exit_code": 0,
  "status": "success"
}
```

## 🐛 故障排查

### 问题 1: 没有可用的 Worker 节点

**错误信息**: `"没有可用的 Worker 节点"`

**解决方案**:
1. 检查 Worker 是否启动
2. 查看 Worker 日志确认心跳正常
3. 检查 Redis 连接是否正常

### 问题 2: 前端页面 404

**解决方案**:
1. 确认前端开发服务器正在运行
2. 检查路由配置是否正确
3. 清除浏览器缓存

### 问题 3: 日志不更新

**解决方案**:
1. 检查 Redis 连接
2. 查看 Worker 日志确认命令在执行
3. 检查前端轮询间隔（应该为 500ms）

### 问题 4: 命令执行超时

**解决方案**:
1. 增加超时时间：`{"command": "...", "timeout": 60}`
2. 检查 Worker 是否在线
3. 查看系统负载

## 📝 测试记录

### 测试环境

- **Master**: localhost:8080
- **Worker**: localhost:9090
- **前端**: localhost:5173
- **Redis**: localhost:6379

### 测试命令

| 命令 | 预期输出 | 预期退出码 |
|------|---------|-----------|
| `pwd` | 当前目录路径 | 0 |
| `whoami` | 当前用户名 | 0 |
| `echo test` | test | 0 |
| `ls /nonexistent` | No such file | 非 0 |
| `sleep 1` | (等待 1 秒) | 0 |
| `date` | 当前日期时间 | 0 |

---

**测试时间**: _______________
**测试人员**: _______________
**测试结果**: ☐ 通过  ☐ 失败（备注: _________）
