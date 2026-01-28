# 测试脚本路径问题修复说明

## 问题描述

之前的测试脚本存在路径矛盾：

```bash
# 问题代码
if [ ! -f "../config.yaml" ]; then  # 检查 ../config.yaml
    echo "配置文件不存在"
    exit 1
fi

cd test  # 又 cd 进入 test 目录！
```

**矛盾点：**
- 脚本已经在 `test/` 目录中
- 检查 `../config.yaml`（相对于 test/ 是正确的）
- 但又执行 `cd test`（会失败或进入错误目录）

---

## 解决方案

使用绝对路径解析：

```bash
# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 使用绝对路径检查配置文件
if [ ! -f "$PROJECT_ROOT/config.yaml" ]; then
    echo "❌ 配置文件 $PROJECT_ROOT/config.yaml 不存在"
    exit 1
fi

# 使用 SCRIPT_DIR 而不是相对路径
cd "$SCRIPT_DIR"
```

---

## 修复的文件

- ✅ [test/run_worker_test.sh](test/run_worker_test.sh)
- ✅ [test/run_e2e_test.sh](test/run_e2e_test.sh)
- ✅ [test/run_integration_test.sh](test/run_integration_test.sh)

---

## 测试验证

可以从任何位置运行测试脚本：

```bash
# 方式 1: 从项目根目录
./test/run_worker_test.sh

# 方式 2: 从 test 目录
cd test && ./run_worker_test.sh

# 方式 3: 使用绝对路径
/codespace/.../test/run_worker_test.sh
```

所有方式都能正确工作！

---

## 技术细节

### 路径解析逻辑

```bash
# 1. 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 2. 获取项目根目录（脚本目录的上一级）
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# 3. 结果示例：
# SCRIPT_DIR  = /path/to/cronicle-next/test
# PROJECT_ROOT = /path/to/cronicle-next
# CONFIG_PATH = /path/to/cronicle-next/config.yaml
```

### BASH_SOURCE 说明

- `${BASH_SOURCE[0]}`: 获取当前脚本的路径
- `dirname`: 提取目录部分
- `cd ... && pwd`: 切换到目录并获取绝对路径

这种方法比相对路径更可靠，因为：
- ✅ 不依赖当前工作目录
- ✅ 支持从任何位置调用脚本
- ✅ 支持符号链接
- ✅ 路径明确，易于调试

---

## 其他改进

同时修复了以下问题：
- ✅ Go 自动检测（支持多个常见安装路径）
- ✅ Redis 自动启动（集成测试）
- ✅ 更清晰的错误提示
- ✅ 统一的脚本风格
