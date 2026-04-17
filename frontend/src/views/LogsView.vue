<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { eventsApi, shellApi, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Card from 'primevue/card'
import ProgressSpinner from 'primevue/progressspinner'

const route = useRoute()
const router = useRouter()
const wsStore = useWebSocketStore()

const loading = ref(false)
const event = ref<Event | null>(null)
const logs = ref('')
const exitCode = ref<number | null>(null)

const eventId = () => route.params.id as string

const LOG_LIMIT = 1000

const getStatusSeverity = (status: string) => {
  const map: Record<string, string> = {
    success: 'success',
    failed: 'danger',
    running: 'warn',
    queued: 'info',
    pending: 'info',
    aborted: 'info',
  }
  return map[status] || 'info'
}

const loadEvent = async () => {
  const id = eventId()
  if (!id) return
  event.value = await eventsApi.get(id)
}

const loadLogs = async () => {
  const id = eventId()
  if (!id) return

  const res = await shellApi.getLogs(id)
  logs.value = res.logs || ''
  exitCode.value = res.exit_code
  if (!logs.value && res.error_message) {
    logs.value = `[错误] ${res.error_message}`
  }
}

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

const isCompleted = computed(() => {
  if (!event.value) return false
  return ['success', 'failed', 'aborted', 'timeout'].includes(event.value.status)
})

const downloadLog = async () => {
  const id = eventId()
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

const reloadAll = async () => {
  try {
    loading.value = true
    await Promise.all([loadEvent(), loadLogs()])
  } catch (error: any) {
    showToast({ severity: 'error', summary: '加载日志失败', detail: error.response?.data?.error || '加载日志失败', life: 5000 })
  } finally {
    loading.value = false
  }
}

const handleLog = (data: any) => {
  if (data.event_id === eventId()) {
    logs.value += data.content || ''
  }
}

const handleHistoryLog = (data: any) => {
  if (data.event_id === eventId() && data.logs) {
    logs.value = data.logs
  }
}

const handleTaskStatus = async (data: any) => {
  if (data.event_id === eventId()) {
    if (event.value) {
      event.value = { ...event.value, status: data.status, exit_code: data.exit_code }
    }
    if (data.status !== 'running') {
      exitCode.value = data.exit_code
      try {
        const res = await shellApi.getLogs(eventId())
        if (res.logs) {
          logs.value = res.logs
        }
      } catch {}
    }
  }
}

onMounted(async () => {
  await reloadAll()

  wsStore.onMessage('log', handleLog)
  wsStore.onMessage('history_log', handleHistoryLog)
  wsStore.onMessage('task_status', handleTaskStatus)
  wsStore.joinRoom(`event:${eventId()}`)
})

onUnmounted(() => {
  wsStore.offMessage('log', handleLog)
  wsStore.offMessage('history_log', handleHistoryLog)
  wsStore.offMessage('task_status', handleTaskStatus)
  wsStore.leaveRoom(`event:${eventId()}`)
})
</script>

<template>
  <div class="logs-page">
    <div v-if="loading" class="flex justify-center py-16">
      <ProgressSpinner style="width:50px;height:50px" strokeWidth="4" />
    </div>

    <template v-else>
      <div class="page-header">
        <div class="left-area">
          <Button icon="pi pi-arrow-left" text @click="router.back()" label="返回" />
          <h2 class="page-title">日志查看</h2>
        </div>
        <div class="right-area">
          <Button icon="pi pi-refresh" text @click="reloadAll" label="刷新" />
        </div>
      </div>

      <Card v-if="event" class="meta-card">
        <template #content>
          <div class="desc-grid">
            <div class="desc-label">Event ID</div>
            <div class="desc-value">{{ event.id }}</div>
            <div class="desc-label">Job ID</div>
            <div class="desc-value">{{ event.job_id }}</div>
            <div class="desc-label">状态</div>
            <div class="desc-value">
              <Tag :value="event.status" :severity="getStatusSeverity(event.status)" />
            </div>
            <div class="desc-label">节点</div>
            <div class="desc-value">{{ event.node_name || event.node_id || '-' }}</div>
            <div class="desc-label">退出码</div>
            <div class="desc-value">{{ exitCode ?? '-' }}</div>
            <div class="desc-label">结束时间</div>
            <div class="desc-value">{{ event.end_time ? new Date(event.end_time).toLocaleString('zh-CN') : '-' }}</div>
            <div v-if="event.error_message" class="desc-label">错误信息</div>
            <div v-if="event.error_message" class="desc-value col-span-2 text-red-500">{{ event.error_message }}</div>
          </div>
        </template>
      </Card>

      <Card class="logs-card">
        <template #header>
          <div class="logs-card-header">
            <span class="logs-stats">
              {{ totalLines }} 行 · {{ logSizeText }}
            </span>
            <Button
              v-if="isCompleted && logs"
              severity="info"
              size="small"
              icon="pi pi-download"
              @click="downloadLog"
              label="下载日志"
            />
          </div>
        </template>
        <template #content>
          <pre v-if="displayLogs" class="log-content">{{ displayLogs }}</pre>
          <div v-else class="text-center py-8 text-gray-400">
            <i class="pi pi-inbox text-4xl mb-2 block"></i>
            <p>暂无日志输出</p>
          </div>
        </template>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.logs-page {
  padding: 24px;
  max-width: 1500px;
  margin: 0 auto;
}

.page-header,
.left-area,
.right-area {
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-header {
  justify-content: space-between;
  margin-bottom: 20px;
}

.page-title {
  margin: 0;
  font-size: 24px;
}

.meta-card,
.logs-card {
  border-radius: 12px;
  margin-bottom: 16px;
}

.desc-grid {
  display: grid;
  grid-template-columns: auto 1fr auto 1fr auto 1fr;
  gap: 0;
}

.desc-label {
  padding: 8px 16px;
  background: #f8fafc;
  font-weight: 500;
  font-size: 13px;
  color: #64748b;
  border-bottom: 1px solid #e2e8f0;
  border-right: 1px solid #e2e8f0;
}

.desc-value {
  padding: 8px 16px;
  font-size: 13px;
  border-bottom: 1px solid #e2e8f0;
}

.col-span-2 {
  grid-column: span 2;
}

.logs-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.logs-stats {
  font-size: 13px;
  color: #64748b;
}

.log-content {
  margin: 0;
  min-height: 420px;
  max-height: 70vh;
  overflow: auto;
  padding: 16px;
  background: #0f172a;
  color: #e2e8f0;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
