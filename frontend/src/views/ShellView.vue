<script setup lang="ts">
import { ref, onMounted, onUnmounted, shallowRef } from 'vue'
import { shellApi, nodesApi, type ShellLogsResponse, type Node } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { VideoPlay, CircleClose, Delete, Loading, QuestionFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { VueCodemirror as Codemirror } from 'codemirror-editor-vue3'
import 'codemirror/addon/display/placeholder.js'
import 'codemirror/mode/shell/shell.js'

// 状态管理
const command = ref('')
const isExecuting = ref(false)
const currentEventId = ref('')
const logs = ref('')
const exitCode = ref<number>(0)
const strictMode = ref(false)
const wsStore = useWebSocketStore()

// 目标服务器
const selectedNodeId = ref('')
const nodes = ref<Node[]>([])
const loadingNodes = ref(false)

// CodeMirror 配置
const cmOptions = {
  mode: 'text/x-sh',  // Shell 模式
  theme: 'default',
  lineNumbers: false,
  autofocus: true,
  lineWrapping: true,
  tabSize: 2,
}

// CodeMirror 变化处理
const onCommandChange = (value: string) => {
  command.value = value
}

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

// 组件挂载时连接WebSocket
onMounted(async () => {
  // 加载节点列表
  await loadNodes()

  // 注册消息处理器
  wsStore.onMessage('log', handleLogMessage)
  wsStore.onMessage('history_log', handleHistoryLog)
  wsStore.onMessage('task_status', handleTaskStatus)
  wsStore.onMessage('error', handleErrorMessage)
})

// 加载节点列表（过滤掉 master 节点）
const loadNodes = async () => {
  try {
    loadingNodes.value = true
    const allNodes = await nodesApi.list({ status: 'online' }) as unknown as Node[]
    // 过滤掉 master 节点（tags 为 master 或包含 master）
    nodes.value = allNodes.filter((node: Node) => node.tags !== 'master' && !node.tags?.includes('master'))

    // 如果只有一个节点，自动选择
    if (nodes.value.length === 1) {
      selectedNodeId.value = nodes.value[0].id
    }

    console.log('加载到在线节点:', nodes.value.length, '个', nodes.value.map(n => ({ id: n.id, tags: n.tags })))
  } catch (error) {
    console.error('加载节点列表失败:', error)
    ElMessage.warning('加载节点列表失败，请检查服务器连接')
  } finally {
    loadingNodes.value = false
  }
}

// 组件卸载时清理
onUnmounted(() => {
  // 取消订阅当前任务
  if (currentEventId.value) {
    wsStore.leaveRoom(`event:${currentEventId.value}`)
  }

  // 移除消息处理器
  wsStore.offMessage('log', handleLogMessage)
  wsStore.offMessage('history_log', handleHistoryLog)
  wsStore.offMessage('task_status', handleTaskStatus)
  wsStore.offMessage('error', handleErrorMessage)
})

// 处理实时日志消息
const handleLogMessage = (data: any) => {
  if (data.event_id === currentEventId.value) {
    // 追加新日志内容
    logs.value += data.content
  }
}

// 处理历史日志消息
const handleHistoryLog = (data: any) => {
  if (data.event_id === currentEventId.value) {
    // 加载完整历史日志
    if (data.logs) {
      logs.value = data.logs
    }
  }
}

// 处理任务状态变化
const handleTaskStatus = (data: any) => {
  if (data.event_id === currentEventId.value && data.status !== 'running') {
    // 任务完成
    isExecuting.value = false
    exitCode.value = data.exit_code

    // 取消订阅
    wsStore.leaveRoom(`event:${currentEventId.value}`)

    // 显示完成消息
    if (data.exit_code === 0) {
      ElMessage.success('命令执行成功')
    } else {
      ElMessage.warning(`命令执行完成，退出码: ${data.exit_code}`)
    }
  }
}

// 处理错误消息
const handleErrorMessage = (data: any) => {
  ElMessage.error('WebSocket错误: ' + data.message)
}

// 执行命令
const executeCommand = async () => {
  if (!command.value.trim()) {
    ElMessage.warning('请输入命令')
    return
  }

  // 检查是否选择了服务器（如果有多个节点）
  if (nodes.value.length > 1 && !selectedNodeId.value) {
    ElMessage.warning('请选择执行服务器')
    return
  }

  try {
    isExecuting.value = true
    logs.value = ''
    exitCode.value = 0

    // 提交命令执行请求
    const response = await shellApi.execute({
      command: command.value,
      node_id: selectedNodeId.value,
      timeout: 3600,
      strict_mode: strictMode.value,
    })

    // 处理API响应 - axios拦截器已经返回了response.data
    const result = (response as any).data || response
    currentEventId.value = result.event_id

    console.log('命令已提交，事件ID:', currentEventId.value)

    // 订阅任务日志
    wsStore.joinRoom(`event:${currentEventId.value}`)
  } catch (error: any) {
    console.error('执行命令失败:', error)
    ElMessage.error('执行命令失败: ' + (error.response?.data?.error || error.message))
    isExecuting.value = false
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

// 获取节点名称
const getNodeName = (nodeId: string) => {
  const node = nodes.value.find(n => n.id === nodeId)
  return node ? `${node.hostname} (${node.ip})` : nodeId
}
</script>

<template>
  <div class="shell-page">
    <!-- 页面头部 -->
    <div class="page-header">
    </div>

    <div class="shell-container">
      <!-- 无节点提示 -->
      <el-card v-if="!loadingNodes && nodes.length === 0" class="command-card" shadow="never">
        <div class="no-nodes">
          <el-icon :size="48"><CircleClose /></el-icon>
          <h3>暂无可用服务器</h3>
          <p>请确保至少有一个 Worker 节点在线</p>
          <el-button type="primary" @click="loadNodes">重新加载</el-button>
        </div>
      </el-card>

      <!-- 命令输入区域 -->
      <el-card v-else class="command-card" shadow="never">
        <template #header>
          <div class="card-header">
            <h3 class="card-title">命令输入</h3>
            <el-tag v-if="isExecuting" type="warning">
              <el-icon class="is-loading"><Loading /></el-icon>
              执行中...
            </el-tag>
          </div>
        </template>

        <!-- 服务器选择 -->
        <div v-if="nodes.length > 0" class="server-selector">
          <div class="server-selector-label">
            <el-icon><VideoPlay /></el-icon>
            <span>目标服务器:</span>
          </div>
          <el-select
            v-model="selectedNodeId"
            placeholder="选择执行命令的服务器"
            :loading="loadingNodes"
            :disabled="isExecuting"
            size="large"
            style="width: 100%"
          >
            <el-option
              v-for="node in nodes"
              :key="node.id"
              :label="`${node.hostname} (${node.ip})`"
              :value="node.id"
            >
              <div style="display: flex; justify-content: space-between; align-items: center">
                <div>
                  <span style="font-weight: 500">{{ node.hostname }}</span>
                  <span style="color: #8492a6; font-size: 12px; margin-left: 8px">{{ node.ip }}</span>
                </div>
                <el-tag size="small" :type="node.running_jobs > 0 ? 'warning' : 'success'">
                  负载: {{ node.running_jobs }}
                </el-tag>
              </div>
            </el-option>
          </el-select>
        </div>

        <!-- 执行选项 -->
        <div class="execution-options">
          <el-checkbox v-model="strictMode" border>
            严格模式
            <el-tooltip content="开启后，命令序列中任何一个命令失败都会立即停止执行" placement="top">
              <el-icon class="help-icon"><QuestionFilled /></el-icon>
            </el-tooltip>
          </el-checkbox>
        </div>

<!-- 命令输入框 -->
        <div class="command-input-group">
          <Codemirror
            v-model:value="command"
            :options="cmOptions"
            :placeholder="'输入要执行的 Shell 命令'"
            :height="'240px'"
            @change="onCommandChange"
          />
          <div class="command-actions">
            <el-button
              type="primary"
              :icon="VideoPlay"
              :loading="isExecuting"
              :disabled="!command.trim()"
              @click="executeCommand"
            >
              执行
            </el-button>
          </div>
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
          <span v-if="selectedNodeId" class="status-item">
            <strong>执行节点:</strong> {{ getNodeName(selectedNodeId) }}
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

.no-nodes {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 20px;
  text-align: center;
  color: #64748b;
}

.no-nodes .el-icon {
  margin-bottom: 16px;
  opacity: 0.5;
}

.no-nodes h3 {
  font-size: 18px;
  font-weight: 600;
  color: #1e293b;
  margin: 0 0 8px 0;
}

.no-nodes p {
  font-size: 14px;
  color: #94a3b8;
  margin: 0 0 20px 0;
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

.server-selector {
  margin-bottom: 20px;
  padding: 16px;
  background: #f8fafc;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
}

.server-selector-label {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 12px;
  font-size: 14px;
  font-weight: 500;
  color: #475569;
}

.command-input-group {
  margin-bottom: 20px;
}

.command-input-group :deep(.CodeMirror) {
  border: 1px solid #c0c4cc !important;
  border-radius: 8px;
  background-color: #fff !important;
  width: 100% !important;
  height: 240px !important;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 14px;
  padding: 4px;
}

.command-input-group :deep(.CodeMirror-scroll) {
  border-radius: 8px;
}

.command-actions {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}

.execution-options {
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.option-hint {
  font-size: 12px;
  color: #94a3b8;
}

.help-icon {
  margin-left: 4px;
  vertical-align: middle;
  color: #94a3b8;
  cursor: help;
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
