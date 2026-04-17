<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { shellApi, nodesApi, eventsApi, type Node } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Card from 'primevue/card'
import Select from 'primevue/select'
import Checkbox from 'primevue/checkbox'
import { VueCodemirror as Codemirror } from 'codemirror-editor-vue3'
import 'codemirror/addon/display/placeholder.js'
import 'codemirror/mode/shell/shell.js'

const command = ref('')
const isExecuting = ref(false)
const currentEventId = ref('')
const logs = ref('')
const exitCode = ref<number>(0)
const strictMode = ref(false)
const wsStore = useWebSocketStore()

const selectedNodeId = ref('')
const nodes = ref<Node[]>([])
const loadingNodes = ref(false)

const cmOptions = {
  mode: 'text/x-sh',
  theme: 'default',
  lineNumbers: false,
  autofocus: true,
  lineWrapping: true,
  tabSize: 2,
}

const onCommandChange = (value: string) => {
  command.value = value
}

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

onMounted(async () => {
  await loadNodes()

  wsStore.onMessage('log', handleLogMessage)
  wsStore.onMessage('history_log', handleHistoryLog)
  wsStore.onMessage('task_status', handleTaskStatus)
  wsStore.onMessage('error', handleErrorMessage)
})

const loadNodes = async () => {
  try {
    loadingNodes.value = true
    const allNodes = await nodesApi.list({ status: 'online' }) as unknown as Node[]
    nodes.value = allNodes.filter((node: Node) => node.tags !== 'master' && !node.tags?.includes('master'))

    if (nodes.value.length === 1) {
      selectedNodeId.value = nodes.value[0].id
    }
  } catch {
    showToast({ severity: 'warn', summary: '加载节点列表失败，请检查服务器连接', life: 3000 })
  } finally {
    loadingNodes.value = false
  }
}

onUnmounted(() => {
  if (currentEventId.value) {
    wsStore.leaveRoom(`event:${currentEventId.value}`)
  }

  wsStore.offMessage('log', handleLogMessage)
  wsStore.offMessage('history_log', handleHistoryLog)
  wsStore.offMessage('task_status', handleTaskStatus)
  wsStore.offMessage('error', handleErrorMessage)
})

const handleLogMessage = (data: any) => {
  if (data.event_id === currentEventId.value) {
    logs.value += data.content
  }
}

const handleHistoryLog = (data: any) => {
  if (data.event_id === currentEventId.value) {
    if (data.logs) {
      logs.value = data.logs
    }
  }
}

const handleTaskStatus = async (data: any) => {
  if (data.event_id === currentEventId.value && data.status !== 'running') {
    isExecuting.value = false
    exitCode.value = data.exit_code

    wsStore.leaveRoom(`event:${currentEventId.value}`)

    try {
      const res = (await shellApi.getLogs(currentEventId.value)) as any
      if (res.logs) {
        logs.value = res.logs
      }
    } catch {}

    if (data.exit_code === 0) {
      showToast({ severity: 'success', summary: '命令执行成功', life: 3000 })
    } else {
      showToast({ severity: 'warn', summary: `命令执行完成，退出码: ${data.exit_code}`, life: 3000 })
    }
  }
}

const handleErrorMessage = (data: any) => {
  showToast({ severity: 'error', summary: 'WebSocket错误', detail: data.message, life: 5000 })
}

const executeCommand = async () => {
  if (!command.value.trim()) {
    showToast({ severity: 'warn', summary: '请输入命令', life: 3000 })
    return
  }

  if (nodes.value.length > 1 && !selectedNodeId.value) {
    showToast({ severity: 'warn', summary: '请选择执行服务器', life: 3000 })
    return
  }

  try {
    isExecuting.value = true
    logs.value = ''
    exitCode.value = 0

    const response = await shellApi.execute({
      command: command.value,
      node_id: selectedNodeId.value,
      timeout: 3600,
      strict_mode: strictMode.value,
    })

    const result = (response as any).data || response
    currentEventId.value = result.event_id

    wsStore.joinRoom(`event:${currentEventId.value}`)
  } catch (error: any) {
    showToast({ severity: 'error', summary: '执行命令失败', detail: error.response?.data?.error || error.message, life: 5000 })
    isExecuting.value = false
  }
}

const abortCommand = async () => {
  if (!currentEventId.value) return
  try {
    await eventsApi.abort(currentEventId.value)
    showToast({ severity: 'success', summary: '已发送中断请求', life: 3000 })
  } catch (error: any) {
    showToast({ severity: 'error', summary: '中断失败', detail: error.response?.data?.error || error.message, life: 5000 })
  }
}

const clearOutput = () => {
  logs.value = ''
  exitCode.value = 0
  currentEventId.value = ''
}

const useQuickCommand = (cmd: string) => {
  command.value = cmd
}

const getNodeName = (nodeId: string) => {
  const node = nodes.value.find(n => n.id === nodeId)
  return node ? `${node.hostname} (${node.ip})` : nodeId
}

const LOG_LIMIT = 1000

const totalLines = computed(() => {
  if (!logs.value) return 0
  const text = logs.value.endsWith('\n') ? logs.value : logs.value.slice(0, logs.value.lastIndexOf('\n') + 1)
  if (!text) return 0
  return text.split('\n').length - 1
})

const displayLogs = computed(() => {
  if (!logs.value) return ''
  let text = logs.value
  if (!text.endsWith('\n')) {
    const lastNL = text.lastIndexOf('\n')
    if (lastNL < 0) return ''
    text = text.slice(0, lastNL)
  }
  const lines = text.split('\n')
  lines.pop()
  if (lines.length <= LOG_LIMIT) return lines.join('\n')
  return lines.slice(-LOG_LIMIT).join('\n')
})

const logSizeText = computed(() => {
  const bytes = new Blob([logs.value]).size
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
})

const downloadLog = async () => {
  const id = currentEventId.value
  if (!id) return
  try {
    const res = await eventsApi.download(id)
    const blob = new Blob([res as any], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${id}.log`
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    showToast({ severity: 'error', summary: '下载日志失败', life: 5000 })
  }
}
</script>

<template>
  <div class="shell-page">
    <div class="shell-container">
      <!-- 无节点提示 -->
      <Card v-if="!loadingNodes && nodes.length === 0" class="command-card">
        <template #content>
          <div class="no-nodes">
            <i class="pi pi-times-circle text-5xl text-gray-300 mb-4 block"></i>
            <h3>暂无可用服务器</h3>
            <p>请确保至少有一个 Worker 节点在线</p>
            <Button severity="info" @click="loadNodes" label="重新加载" />
          </div>
        </template>
      </Card>

      <!-- 命令输入区域 -->
      <Card v-else class="command-card">
        <template #title>
          <div class="card-header">
            <h3 class="card-title">命令输入</h3>
            <Tag v-if="isExecuting" severity="warn">
              <i class="pi pi-spin pi-spinner mr-1"></i>
              执行中...
            </Tag>
          </div>
        </template>
        <template #content>
          <!-- 执行配置行 -->
          <div class="shell-controls">
            <div v-if="nodes.length > 0" class="control-item">
              <span class="control-label">目标服务器:</span>
              <Select
                v-model="selectedNodeId"
                :options="nodes"
                optionLabel="hostname"
                optionValue="id"
                placeholder="选择服务器"
                :loading="loadingNodes"
                :disabled="isExecuting"
                class="w-72"
              >
                <template #value="{ value }">
                  <span v-if="value">{{ nodes.find(n => n.id === value)?.hostname }} ({{ nodes.find(n => n.id === value)?.ip }})</span>
                </template>
                <template #option="{ option }">
                  <div class="flex justify-between items-center w-full">
                    <div>
                      <span class="font-medium">{{ option.hostname }}</span>
                      <span class="text-gray-400 text-xs ml-2">{{ option.ip }}</span>
                    </div>
                    <Tag :value="`负载: ${option.running_jobs}`" :severity="option.running_jobs > 0 ? 'warn' : 'success'" />
                  </div>
                </template>
              </Select>
            </div>

            <div class="control-item">
              <div class="flex items-center gap-2">
                <Checkbox v-model="strictMode" inputId="strictMode" binary />
                <label for="strictMode" class="text-sm">严格模式</label>
                <i class="pi pi-question-circle help-icon cursor-help" v-tooltip.top="'开启后，命令序列中任何一个命令失败都会立即停止执行'"></i>
              </div>
            </div>
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
              <Button
                v-if="isExecuting"
                severity="danger"
                icon="pi pi-stop-circle"
                @click="abortCommand"
                label="中断"
              />
              <Button
                severity="info"
                icon="pi pi-play"
                :loading="isExecuting"
                :disabled="!command.trim()"
                @click="executeCommand"
                label="执行"
              />
            </div>
          </div>

          <!-- 快速命令 -->
          <div class="quick-commands">
            <div class="quick-commands-label">快速命令:</div>
            <div class="quick-commands-list">
              <Button
                v-for="cmd in quickCommands"
                :key="cmd.cmd"
                size="small"
                text
                severity="secondary"
                :disabled="isExecuting"
                @click="useQuickCommand(cmd.cmd)"
              >
                {{ cmd.name }}
              </Button>
            </div>
          </div>
        </template>
      </Card>

      <!-- 输出区域 -->
      <Card class="output-card">
        <template #title>
          <div class="card-header">
            <div class="card-header-left">
              <h3 class="card-title">执行输出</h3>
              <span v-if="logs" class="logs-stats">{{ totalLines }} 行 · {{ logSizeText }}</span>
            </div>
            <div class="card-actions">
              <Tag v-if="exitCode !== 0" severity="danger">退出码: {{ exitCode }}</Tag>
              <Tag v-else-if="logs" severity="success">退出码: 0</Tag>
              <Button
                v-if="!isExecuting && logs && currentEventId"
                severity="info"
                size="small"
                icon="pi pi-download"
                @click="downloadLog"
                label="下载"
              />
              <Button
                icon="pi pi-trash"
                size="small"
                text
                severity="secondary"
                :disabled="isExecuting || !logs"
                @click="clearOutput"
                label="清空"
              />
            </div>
          </div>
        </template>
        <template #content>
          <!-- 日志输出 -->
          <div class="logs-container">
            <pre v-if="displayLogs" class="logs-content">{{ displayLogs }}</pre>
            <div v-else class="logs-empty">
              <i class="pi pi-times-circle text-5xl text-gray-300"></i>
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
              <i class="pi pi-spin pi-spinner"></i>
              正在执行...
            </span>
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>

<style scoped>
.shell-page {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.shell-container {
  display: grid;
  gap: 20px;
}

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

.card-header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logs-stats {
  font-size: 13px;
  color: #64748b;
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

.shell-controls {
  display: flex;
  align-items: center;
  gap: 24px;
  margin-bottom: 20px;
  padding: 12px 16px;
  background: #f8fafc;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.control-item {
  display: flex;
  align-items: center;
  gap: 12px;
}

.control-label {
  font-size: 14px;
  font-weight: 600;
  color: #475569;
  white-space: nowrap;
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

.help-icon {
  color: #94a3b8;
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

@media (max-width: 768px) {
  .shell-page {
    padding: 16px;
  }

  .quick-commands {
    flex-direction: column;
  }

  .quick-commands-label {
    padding-top: 0;
  }
}
</style>
