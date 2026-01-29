<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import { shellApi, type ShellLogsResponse } from '@/api'
import { VideoPlay, CircleClose, Delete, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

// 状态管理
const command = ref('')
const isExecuting = ref(false)
const currentEventId = ref('')
const logs = ref('')
const exitCode = ref<number>(0)
const pollTimer = ref<number>()

// 快速命令示例
const quickCommands = [
  { name: '当前目录', cmd: 'pwd' },
  { name: '列出文件', cmd: 'ls -la' },
  { name: '当前用户', cmd: 'whoami' },
  { name: '系统时间', cmd: 'date' },
  { name: '系统信息', cmd: 'uname -a' },
  { name: '磁盘使用', cmd: 'df -h' },
  { name: '内存使用', cmd: 'free -h' },
  { name: 'CPU 信息', cmd: 'cat /proc/cpuinfo | grep "model name" | head -1' },
]

// 执行命令
const executeCommand = async () => {
  if (!command.value.trim()) {
    ElMessage.warning('请输入命令')
    return
  }

  try {
    isExecuting.value = true
    logs.value = ''
    exitCode.value = 0

    // 提交命令执行请求
    const response = await shellApi.execute({
      command: command.value,
      timeout: 30,
    })

    currentEventId.value = response.event_id

    // 开始轮询日志
    startPolling(response.event_id)
  } catch (error: any) {
    ElMessage.error('执行命令失败: ' + (error.response?.data?.error || error.message))
    isExecuting.value = false
  }
}

// 轮询获取日志
const startPolling = (eventId: string) => {
  pollTimer.value = window.setInterval(async () => {
    try {
      const result: ShellLogsResponse = await shellApi.getLogs(eventId)

      // 更新日志
      if (result.logs && result.logs !== logs.value) {
        logs.value = result.logs
      }

      // 检查是否完成
      if (result.complete) {
        stopPolling()
        exitCode.value = result.exit_code
        isExecuting.value = false

        if (result.exit_code === 0) {
          ElMessage.success('命令执行成功')
        } else {
          ElMessage.warning(`命令执行完成，退出码: ${result.exit_code}`)
        }
      }
    } catch (error: any) {
      console.error('获取日志失败:', error)
      // 不中断轮询，可能是临时网络问题
    }
  }, 500) // 每 500ms 轮询一次
}

// 停止轮询
const stopPolling = () => {
  if (pollTimer.value) {
    clearInterval(pollTimer.value)
    pollTimer.value = undefined
  }
}

// 清空输出
const clearOutput = () => {
  logs.value = ''
  exitCode.value = 0
  currentEventId.value = ''
}

// 使用快速命令
const useQuickCommand = (cmd: string) => {
  command.value = cmd
}

// 组件卸载时清理
onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div class="shell-page">
    <!-- 页面标题 -->
    <div class="page-header">
      <div>
        <h1 class="page-title">Shell 执行</h1>
        <p class="page-subtitle">在 Worker 节点上执行 Shell 命令并查看实时输出</p>
      </div>
    </div>

    <div class="shell-container">
      <!-- 命令输入区域 -->
      <el-card class="command-card" shadow="never">
        <template #header>
          <div class="card-header">
            <h3 class="card-title">命令输入</h3>
            <el-tag v-if="isExecuting" type="warning">
              <el-icon class="is-loading"><Loading /></el-icon>
              执行中...
            </el-tag>
          </div>
        </template>

        <!-- 命令输入框 -->
        <div class="command-input-group">
          <el-input
            v-model="command"
            placeholder="输入要执行的 Shell 命令，例如: ls -la"
            size="large"
            :disabled="isExecuting"
            @keyup.enter="executeCommand"
            clearable
          >
            <template #append>
              <el-button
                type="primary"
                :icon="VideoPlay"
                :loading="isExecuting"
                :disabled="!command.trim()"
                @click="executeCommand"
              >
                执行
              </el-button>
            </template>
          </el-input>
        </div>

        <!-- 快速命令 -->
        <div class="quick-commands">
          <div class="quick-commands-label">快速命令:</div>
          <div class="quick-commands-list">
            <el-button
              v-for="cmd in quickCommands"
              :key="cmd.cmd"
              size="small"
              :disabled="isExecuting"
              @click="useQuickCommand(cmd.cmd)"
            >
              {{ cmd.name }}
            </el-button>
          </div>
        </div>
      </el-card>

      <!-- 输出区域 -->
      <el-card class="output-card" shadow="never">
        <template #header>
          <div class="card-header">
            <h3 class="card-title">执行输出</h3>
            <div class="card-actions">
              <el-tag v-if="exitCode !== 0" type="danger">退出码: {{ exitCode }}</el-tag>
              <el-tag v-else-if="logs" type="success">退出码: 0</el-tag>
              <el-button
                :icon="Delete"
                size="small"
                :disabled="isExecuting || !logs"
                @click="clearOutput"
              >
                清空
              </el-button>
            </div>
          </div>
        </template>

        <!-- 日志输出 -->
        <div class="logs-container">
          <pre v-if="logs" class="logs-content">{{ logs }}</pre>
          <div v-else class="logs-empty">
            <el-icon :size="48"><CircleClose /></el-icon>
            <p>暂无输出</p>
            <p class="logs-empty-hint">输入命令并点击执行按钮查看结果</p>
          </div>
        </div>

        <!-- 状态信息 -->
        <div v-if="currentEventId" class="status-bar">
          <span class="status-item">
            <strong>Event ID:</strong> {{ currentEventId }}
          </span>
          <span v-if="isExecuting" class="status-item status-running">
            <el-icon class="is-loading"><Loading /></el-icon>
            正在执行...
          </span>
        </div>
      </el-card>
    </div>
  </div>
</template>

<style scoped>
.shell-page {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
  margin: 0 0 4px 0;
}

.page-subtitle {
  font-size: 14px;
  color: #64748b;
  margin: 0;
}

.shell-container {
  display: grid;
  gap: 20px;
}

/* 命令输入卡片 */
.command-card {
  border-radius: 16px;
  border: 1px solid #e2e8f0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.command-input-group {
  margin-bottom: 20px;
}

.quick-commands {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  flex-wrap: wrap;
}

.quick-commands-label {
  font-size: 14px;
  font-weight: 500;
  color: #64748b;
  padding-top: 6px;
}

.quick-commands-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

/* 输出卡片 */
.output-card {
  border-radius: 16px;
  border: 1px solid #e2e8f0;
}

.logs-container {
  min-height: 400px;
  max-height: 600px;
  overflow: auto;
  background: #1e293b;
  border-radius: 8px;
  padding: 16px;
}

.logs-content {
  margin: 0;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  color: #f8fafc;
  white-space: pre-wrap;
  word-break: break-all;
}

.logs-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 300px;
  color: #64748b;
  gap: 12px;
}

.logs-empty .el-icon {
  opacity: 0.5;
}

.logs-empty-hint {
  font-size: 13px;
  color: #94a3b8;
}

.status-bar {
  display: flex;
  align-items: center;
  gap: 24px;
  padding-top: 16px;
  border-top: 1px solid #e2e8f0;
  font-size: 13px;
}

.status-item {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #64748b;
}

.status-running {
  color: #3b82f6;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .shell-page {
    padding: 16px;
  }

  .page-title {
    font-size: 24px;
  }

  .quick-commands {
    flex-direction: column;
  }

  .quick-commands-label {
    padding-top: 0;
  }
}
</style>
