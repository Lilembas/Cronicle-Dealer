<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import Button from 'primevue/button'
import Card from 'primevue/card'
import Select from 'primevue/select'
import InputText from 'primevue/inputtext'
import { useWebSocketStore } from '@/stores/websocket'

interface LogEntry {
  level: string
  time: string
  message: string
  fields?: string
}

const logs = ref<LogEntry[]>([])
const autoScroll = ref(true)
const levelFilter = ref('')
const searchText = ref('')
const displayCount = ref(20)
const containerRef = ref<HTMLElement>()

const wsStore = useWebSocketStore()

const levelOptions = [
  { label: '全部', value: '' },
  { label: 'DEBUG', value: 'debug' },
  { label: 'INFO', value: 'info' },
  { label: 'WARN', value: 'warn' },
  { label: 'ERROR', value: 'error' },
]

const countOptions = [
  { label: '最近 20 条', value: 20 },
  { label: '最近 50 条', value: 50 },
  { label: '最近 100 条', value: 100 },
  { label: '最近 200 条', value: 200 },
  { label: '最近 500 条', value: 500 },
  { label: '全部', value: 0 },
]

const filteredLogs = computed(() => {
  let result = logs.value
  if (levelFilter.value) {
    result = result.filter(l => l.level === levelFilter.value)
  }
  if (searchText.value) {
    const keyword = searchText.value.toLowerCase()
    result = result.filter(l => l.message.toLowerCase().includes(keyword))
  }
  return result
})

const displayLogs = computed(() => {
  const count = displayCount.value
  if (count === 0) return filteredLogs.value
  const total = filteredLogs.value.length
  if (total <= count) return filteredLogs.value
  return filteredLogs.value.slice(total - count)
})

function getLevelClass(level: string) {
  switch (level) {
    case 'debug': return 'log-debug'
    case 'info': return 'log-info'
    case 'warn': return 'log-warn'
    case 'error': return 'log-error'
    default: return ''
  }
}

function scrollToBottom() {
  if (autoScroll.value && containerRef.value) {
    nextTick(() => {
      if (containerRef.value) {
        containerRef.value.scrollTop = containerRef.value.scrollHeight
      }
    })
  }
}

function handleManagerLog(data: any) {
  const entry: LogEntry = {
    level: data.level || 'info',
    time: data.time || '',
    message: data.message || '',
    fields: data.fields,
  }
  logs.value.push(entry)
  if (logs.value.length > 5000) {
    logs.value = logs.value.slice(-3000)
  }
  scrollToBottom()
}

onMounted(() => {
  wsStore.onMessage('manager_log', handleManagerLog)
  wsStore.joinRoom('manager:logs')
})

onUnmounted(() => {
  wsStore.leaveRoom('manager:logs')
  wsStore.offMessage('manager_log', handleManagerLog)
})

function clearLogs() {
  logs.value = []
}
</script>

<template>
  <div class="logs-page">
    <Card class="toolbar-card">
      <template #content>
        <div class="flex items-center gap-3 flex-wrap">
          <Select
            v-model="levelFilter"
            :options="levelOptions"
            optionLabel="label"
            optionValue="value"
            placeholder="日志级别"
            style="width: 140px"
          />
          <InputText
            v-model="searchText"
            placeholder="搜索日志..."
            style="width: 240px"
          />
          <Select
            v-model="displayCount"
            :options="countOptions"
            optionLabel="label"
            optionValue="value"
            style="width: 140px"
          />
          <div class="flex-1" />
          <span :class="['connection-dot', wsStore.isConnected ? 'connected' : 'disconnected']">
            {{ wsStore.isConnected ? '已连接' : '未连接' }}
          </span>
          <Button label="自动滚动" :severity="autoScroll ? undefined : 'secondary'" size="small" @click="autoScroll = !autoScroll" />
          <Button label="清空" icon="pi pi-trash" severity="danger" size="small" text @click="clearLogs" />
        </div>
      </template>
    </Card>

    <Card class="log-card">
      <template #content>
        <div ref="containerRef" class="log-container">
          <div v-if="displayLogs.length === 0" class="log-empty">
            <i class="pi pi-file-o text-4xl mb-2 block text-gray-300" />
            <p class="text-gray-400">暂无日志</p>
          </div>
          <div
            v-for="(log, index) in displayLogs"
            :key="index"
            :class="['log-line', getLevelClass(log.level)]"
          >
            <span class="log-time">{{ log.time }}</span>
            <span :class="['log-level', `level-${log.level}`]">{{ log.level.toUpperCase().padEnd(5) }}</span>
            <span class="log-message">{{ log.message }}</span>
            <span v-if="log.fields && log.fields !== '{}'" class="log-fields">{{ log.fields }}</span>
          </div>
        </div>
      </template>
    </Card>
  </div>
</template>

<style scoped>
.toolbar-card :deep(.p-card-content) {
  padding: 0.75rem 1rem;
}

.connection-dot {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.connection-dot::before {
  content: '';
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.connection-dot.connected::before {
  background: #22c55e;
  box-shadow: 0 0 6px rgba(34, 197, 94, 0.5);
}

.connection-dot.connected {
  color: #22c55e;
}

.connection-dot.disconnected::before {
  background: #ef4444;
}

.connection-dot.disconnected {
  color: #ef4444;
}

.log-card :deep(.p-card-content) {
  padding: 0;
}

.log-container {
  height: 600px;
  overflow-y: auto;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  background: #1e1e2e;
  border-radius: 6px;
  padding: 12px;
}

.log-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
}

.log-line {
  display: flex;
  gap: 12px;
  padding: 1px 4px;
  border-radius: 2px;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line:hover {
  background: rgba(255, 255, 255, 0.04);
}

.log-time {
  color: #6c7086;
  flex-shrink: 0;
}

.log-level {
  flex-shrink: 0;
  font-weight: 600;
  width: 50px;
}

.level-debug { color: #89b4fa; }
.level-info { color: #a6e3a1; }
.level-warn { color: #f9e2af; }
.level-error { color: #f38ba8; }

.log-message {
  color: #cdd6f4;
  flex: 1;
}

.log-fields {
  color: #9399b2;
  font-size: 12px;
  opacity: 0.85;
}

.log-container::-webkit-scrollbar {
  width: 8px;
}

.log-container::-webkit-scrollbar-track {
  background: transparent;
}

.log-container::-webkit-scrollbar-thumb {
  background: #45475a;
  border-radius: 4px;
}

.log-container::-webkit-scrollbar-thumb:hover {
  background: #585b70;
}
</style>
