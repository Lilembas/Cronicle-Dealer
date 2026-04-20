<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed, inject, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { eventsApi, shellApi, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Card from 'primevue/card'
import Skeleton from 'primevue/skeleton'
import Breadcrumb from 'primevue/breadcrumb'

const route = useRoute()
const router = useRouter()
const wsStore = useWebSocketStore()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')

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

const breadcrumbItems = computed(() => [
  { label: '执行记录', command: () => router.push('/events') },
  { label: '日志查看' }
])

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
  if (globalRefreshHandler) {
    globalRefreshHandler.value = reloadAll
  }
})

onUnmounted(() => {
  wsStore.offMessage('log', handleLog)
  wsStore.offMessage('history_log', handleHistoryLog)
  wsStore.offMessage('task_status', handleTaskStatus)
  wsStore.leaveRoom(`event:${eventId()}`)
  if (globalRefreshHandler) {
    globalRefreshHandler.value = null
  }
})
</script>

<template>
  <div class="logs-page">
    <div v-if="loading" class="skeleton-page">
      <div class="flex items-center gap-3 mb-6">
        <Skeleton width="60px" height="32px" borderRadius="8px" />
        <Skeleton width="120px" height="24px" />
      </div>
      <Skeleton width="100%" height="160px" borderRadius="12px" class="mb-4" />
      <Skeleton width="100%" height="420px" borderRadius="12px" />
    </div>

    <template v-else>
      <div class="page-header">
        <div class="left-area">
          <Breadcrumb :model="breadcrumbItems" />
        </div>
        <div class="right-area"></div>
      </div>

      <Card v-if="event" class="meta-card">
        <template #content>
          <div class="info-grid">
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-id-card"></i>
                <span>Event ID</span>
              </div>
              <div class="info-value mono">{{ event.id }}</div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-cog"></i>
                <span>Job ID</span>
              </div>
              <div class="info-value mono">{{ event.job_id }}</div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-info-circle"></i>
                <span>状态</span>
              </div>
              <div class="info-value">
                <Tag :value="event.status" :severity="getStatusSeverity(event.status)" />
              </div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-server"></i>
                <span>节点</span>
              </div>
              <div class="info-value">{{ event.node_name || event.node_id || '-' }}</div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-flag"></i>
                <span>退出码</span>
              </div>
              <div class="info-value" :class="{ 'exit-error': exitCode !== 0 && exitCode !== null && exitCode !== undefined }">
                {{ exitCode ?? '-' }}
              </div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-clock"></i>
                <span>结束时间</span>
              </div>
              <div class="info-value">{{ event.end_time ? new Date(event.end_time).toLocaleString('zh-CN') : '-' }}</div>
            </div>
          </div>
          <div v-if="event.error_message" class="error-box">
            <div class="error-header">
              <i class="pi pi-exclamation-triangle"></i>
              <span>错误信息</span>
            </div>
            <div class="error-content">{{ event.error_message }}</div>
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
  max-width: 1400px;
  margin: 0 auto;
}

.page-header,
.left-area,
.right-area {
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-header {
  justify-content: space-between;
  margin-bottom: 20px;
}

.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.meta-card,
.logs-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
  margin-bottom: 16px;
}

/* 信息网格布局 */
.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.info-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--color-text-muted);
}

.info-label i {
  font-size: 12px;
  color: #94a3b8;
}

.info-value {
  font-size: 13px;
  color: var(--color-text-primary);
  font-weight: 500;
}

.info-value.mono {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 13px;
  word-break: break-all;
}

.exit-error {
  color: #dc2626;
}

/* 错误信息框 */
.error-box {
  margin-top: 16px;
  padding: 12px 16px;
  background: #fef2f2;
  border: 1px solid #fecaca;
  border-radius: 8px;
}

.error-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: #dc2626;
  margin-bottom: 6px;
}

.error-header i {
  font-size: 14px;
}

.error-content {
  font-size: 13px;
  color: #991b1b;
  line-height: 1.5;
}

.logs-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
}

.logs-stats {
  font-size: 13px;
  color: var(--color-text-muted);
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
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
