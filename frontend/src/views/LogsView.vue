<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { eventsApi, shellApi, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { ArrowLeft, RefreshRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()
const wsStore = useWebSocketStore()

const loading = ref(false)
const event = ref<Event | null>(null)
const logs = ref('')
const exitCode = ref<number | null>(null)

const eventId = () => route.params.id as string

const getStatusType = (status: string) => {
  const map: Record<string, any> = {
    success: 'success',
    failed: 'danger',
    running: 'warning',
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
  // 如果日志内容为空但存在 error_message，将其追加到日志中展示
  if (!logs.value && res.error_message) {
    logs.value = `[错误] ${res.error_message}`
  }
}

const reloadAll = async () => {
  try {
    loading.value = true
    await Promise.all([loadEvent(), loadLogs()])
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '加载日志失败')
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

const handleTaskStatus = (data: any) => {
  if (data.event_id === eventId()) {
    if (event.value) {
      event.value = { ...event.value, status: data.status, exit_code: data.exit_code }
    }
    if (data.status !== 'running') {
      exitCode.value = data.exit_code
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
  <div class="logs-page" v-loading="loading">
    <div class="page-header">
      <div class="left-area">
        <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
        <h2 class="page-title">日志查看</h2>
      </div>
      <div class="right-area">
        <el-button :icon="RefreshRight" @click="reloadAll">刷新</el-button>
      </div>
    </div>

    <el-card v-if="event" class="meta-card" shadow="never">
      <el-descriptions :column="3" border>
        <el-descriptions-item label="Event ID">{{ event.id }}</el-descriptions-item>
        <el-descriptions-item label="Job ID">{{ event.job_id }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <el-tag size="small" :type="getStatusType(event.status)">
            {{ event.status }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="节点">{{ event.node_name || event.node_id || '-' }}</el-descriptions-item>
        <el-descriptions-item label="退出码">{{ exitCode ?? '-' }}</el-descriptions-item>
        <el-descriptions-item label="结束时间">{{ event.end_time ? new Date(event.end_time).toLocaleString('zh-CN') : '-' }}</el-descriptions-item>
        <el-descriptions-item v-if="event.error_message" label="错误信息" :span="3">
          <el-text type="danger">{{ event.error_message }}</el-text>
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card class="logs-card" shadow="never">
      <pre v-if="logs" class="log-content">{{ logs }}</pre>
      <el-empty v-else description="暂无日志输出" />
    </el-card>
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
