<script setup lang="ts">
import { onMounted, onUnmounted, ref, inject, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { jobsApi, eventsApi, statsApi, type Job, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { useSystemStore } from '@/stores/system'
import { showToast } from '@/utils/toast'
import { showConfirm, hl } from '@/utils/confirm'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Skeleton from 'primevue/skeleton'
import Breadcrumb from 'primevue/breadcrumb'

const wsStore = useWebSocketStore()
const systemStore = useSystemStore()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')

const route = useRoute()
const router = useRouter()
const job = ref<Job | null>(null)
const events = ref<Event[]>([])
const loading = ref(false)
const triggering = ref(false)

const getStatusText = (status: string) => {
  const map: Record<string, string> = {
    success: '成功',
    failed: '失败',
    running: '运行中',
    queued: '已入队',
    pending: '待执行',
    aborted: '已中止',
  }
  return map[status] || status
}

const loadData = async (silent = false) => {
  const id = route.params.id as string
  if (!id) return

  try {
    if (!silent) loading.value = true
    const [jobData, eventsData] = await Promise.all([
      jobsApi.get(id),
      eventsApi.list({ job_id: id, page: 1, page_size: 20 }),
    ])

    job.value = jobData as unknown as Job
    events.value = (eventsData as unknown as { data: Event[] }).data || []
  } catch (error: any) {
    showToast({ severity: 'error', summary: '加载失败', detail: error.response?.data?.error || '加载任务详情失败', life: 5000 })
  } finally {
    loading.value = false
  }
}

const handleTrigger = async () => {
  if (!job.value || triggering.value) return

  showConfirm({
    message: `确定要触发任务 ${hl(job.value.name)} 吗？`,
    header: '确认触发',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: 'info' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        triggering.value = true
        const result = await jobsApi.trigger(job.value!.id) as unknown as import('@/api').TriggerResponse
        showToast({ severity: 'success', summary: '任务已入队', detail: `Event ID: ${result.event_id}`, life: 5000 })
        loadData(true)
      } catch (error: any) {
        showToast({ severity: 'error', summary: '触发失败', detail: error.response?.data?.error || '任务触发失败', life: 5000 })
      } finally {
        triggering.value = false
      }
    },
  })
}

const handleEdit = () => {
  if (job.value) {
    router.push(`/jobs/${job.value.id}`)
  }
}

const viewLog = (id: string) => {
  router.push(`/logs/${id}`)
}

const canAbort = (status: string) => status === 'running' || status === 'pending' || status === 'queued'

const handleAbort = async (event: Event) => {
  showConfirm({
    message: `确认中止任务 ${hl(event.id)} 吗？`,
    header: '中止确认',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确认', severity: 'danger' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        await eventsApi.abort(event.id)
        showToast({ severity: 'success', summary: '中止请求已提交', life: 3000 })
        loadData(true)
      } catch {
        showToast({ severity: 'error', summary: '中止失败', life: 5000 })
      }
    },
  })
}

const breadcrumbItems = ref([
  { label: '任务管理', command: () => router.push('/jobs') },
  { label: '任务详情' }
])

const handleTaskStatus = (data: any) => {
  if (job.value && data.job_id === job.value.id) {
    loadData(true)
  }
}

onMounted(() => {
  loadData()
  wsStore.onMessage('task_status', handleTaskStatus)
  if (globalRefreshHandler) {
    globalRefreshHandler.value = () => loadData(true)
  }
})

onUnmounted(() => {
  wsStore.offMessage('task_status', handleTaskStatus)
  if (globalRefreshHandler) {
    globalRefreshHandler.value = null
  }
})

const formatDuration = (seconds: number) => {
  if (!seconds && seconds !== 0) return '-'
  if (seconds < 60) return `${seconds}秒`
  return `${(seconds / 60).toFixed(1)}分`
}
</script>

<template>
  <div class="job-detail">
    <div v-if="loading" class="skeleton-page">
      <div class="flex items-center gap-3 mb-6">
        <Skeleton width="60px" height="32px" borderRadius="8px" />
        <Skeleton width="120px" height="24px" />
      </div>
      <div class="mb-4">
        <Skeleton width="100%" height="200px" borderRadius="12px" />
      </div>
      <Skeleton width="100%" height="300px" borderRadius="12px" />
    </div>

    <template v-else>
      <div class="page-header">
        <div class="left-actions">
          <Breadcrumb :model="breadcrumbItems" />
        </div>
        <div class="right-actions">
          <Button icon="pi pi-play" :loading="triggering" class="btn-trigger" @click="handleTrigger" label="立即触发" />
          <Button icon="pi pi-pencil" class="btn-edit" @click="handleEdit" label="编辑" />
        </div>
      </div>

      <Card v-if="job" class="job-card">
        <template #title>
          <div class="card-header">
            <span class="job-name">{{ job.name }}</span>
            <Tag :value="job.enabled ? '已启用' : '已禁用'" :severity="job.enabled ? 'success' : 'secondary'" />
          </div>
        </template>
        <template #content>
          <div class="info-grid">
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-id-card"></i>
                <span>Job ID</span>
              </div>
              <div class="info-value mono">{{ job.id }}</div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-tag"></i>
                <span>分类</span>
              </div>
              <div class="info-value">{{ job.category || '-' }}</div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-calendar"></i>
                <span>Cron</span>
              </div>
              <div class="info-value">{{ job.cron_expr }}</div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-cog"></i>
                <span>任务类型</span>
              </div>
              <div class="info-value">{{ job.task_type }}</div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-clock"></i>
                <span>超时（秒）</span>
              </div>
              <div class="info-value">{{ job.timeout }}</div>
            </div>
            <div class="info-item">
              <div class="info-label">
                <i class="pi pi-calendar-plus"></i>
                <span>下次执行</span>
              </div>
              <div class="info-value">{{ job.next_run_time ? new Date(job.next_run_time).toLocaleString('zh-CN') : '-' }}</div>
            </div>
          </div>
          <div v-if="job.description" class="desc-row">
            <div class="desc-label">
              <i class="pi pi-file-edit"></i>
              <span>描述</span>
            </div>
            <div class="desc-value">{{ job.description }}</div>
          </div>
          <div class="desc-row">
            <div class="desc-label">
              <i class="pi pi-terminal"></i>
              <span>执行命令</span>
            </div>
            <pre class="command">{{ job.command }}</pre>
          </div>
        </template>
      </Card>

      <Card class="events-card">
        <template #title>
          <div class="card-header">
            <span>最近执行记录</span>
            <Tag :value="`${events.length} 条`" severity="info" />
          </div>
        </template>
        <template #content>
          <DataTable :value="events" stripedRows class="events-table">
            <Column field="id" header="Event ID" style="width: 140px">
              <template #body="{ data }">
                <span class="link-text event-id" @click="router.push(`/logs/${data.id}`)">{{ data.id.split('_').slice(-1)[0] }}</span>
              </template>
            </Column>

            <Column header="执行节点" style="width: 140px">
              <template #body="{ data }">
                <span v-if="data.node_name" class="target-node">
                  <i class="pi pi-desktop" />
                  <span>{{ data.node_name }}</span>
                </span>
                <span v-else class="text-gray-400">-</span>
              </template>
            </Column>

            <Column header="状态" style="width: 110px" alignHeader="center" align="center">
              <template #body="{ data }">
                <span :class="['premium-badge', `badge-${data.status}`]">
                  <i v-if="data.status === 'success'" class="pi pi-check-circle"></i>
                  <i v-else-if="data.status === 'failed'" class="pi pi-times-circle"></i>
                  <i v-else-if="data.status === 'running'" class="pi pi-spin pi-spinner"></i>
                  <i v-else class="pi pi-clock"></i>
                  <span>{{ getStatusText(data.status) }}</span>
                </span>
              </template>
            </Column>

            <Column header="开始时间" style="width: 180px">
              <template #body="{ data }">
                <span class="time-text">{{ data.start_time ? new Date(data.start_time).toLocaleString('zh-CN') : '-' }}</span>
              </template>
            </Column>
            <Column header="持续时长" style="width: 100px" alignHeader="right" align="right">
              <template #body="{ data }">
                <span v-if="data.status === 'running' && data.start_time" class="time-text font-mono text-blue-500 font-bold">
                  {{ formatDuration(Math.max(0, Math.floor((systemStore.currentTime - new Date(data.start_time).getTime()) / 1000))) }}
                </span>
                <span v-else class="time-text font-mono">{{ data.duration ? formatDuration(data.duration) : '-' }}</span>
              </template>
            </Column>
            <Column header="退出码" style="width: 100px" alignHeader="center" align="center">
              <template #body="{ data }">
                <span v-if="data.exit_code !== undefined" :class="data.exit_code === 0 ? 'text-green' : 'text-red'">
                  {{ data.exit_code }}
                </span>
                <span v-else>-</span>
              </template>
            </Column>
            <Column header="操作" frozen alignFrozen="right" style="width: 100px" align="left">
              <template #body="{ data }">
                <div class="action-buttons">
                  <Button v-tooltip.top="'详情'" size="small" icon="pi pi-eye" severity="info" class="btn-log" @click="viewLog(data.id)" />
                  <Button v-if="canAbort(data.status)" v-tooltip.top="'中止'" size="small" icon="pi pi-stop-circle" severity="danger" class="btn-abort" @click="handleAbort(data)" />
                </div>
              </template>
            </Column>
          </DataTable>
        </template>
      </Card>
    </template>
  </div>
</template>

<style scoped>
.job-detail {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  gap: 12px;
}

.left-actions,
.right-actions,
.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.right-actions :deep(.p-button) {
  padding: 8px 16px;
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  border: 1px solid transparent;
}

.right-actions :deep(.p-button:hover) {
  transform: translateY(-1px);
}

.btn-trigger {
  background: #f0f9ff !important;
  color: #0284c7 !important;
  border-color: #bae6fd !important;
}

.btn-trigger:hover {
  background: #e0f2fe !important;
  border-color: #7dd3fc !important;
  box-shadow: 0 4px 6px -1px rgba(14, 165, 233, 0.1), 0 2px 4px -1px rgba(14, 165, 233, 0.06) !important;
}

.btn-edit {
  background: #f8fafc !important;
  color: #475569 !important;
  border-color: #e2e8f0 !important;
}

.btn-edit:hover {
  background: #f1f5f9 !important;
  border-color: #cbd5e1 !important;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05) !important;
}

.page-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.job-card,
.events-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
  margin-bottom: 16px;
}

.job-name {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.command {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 13px;
  word-break: break-all;
  margin: 0;
  padding: 12px;
  background: #f8fafc;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
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

/* 描述和命令行 */
.desc-row {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #e2e8f0;
}

.desc-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--color-text-muted);
  margin-bottom: 8px;
}

.desc-label i {
  font-size: 12px;
  color: #94a3b8;
}

/* 执行记录表格 */
.events-table {
  width: 100%;
}

.action-buttons {
  display: flex;
  gap: 4px;
  justify-content: flex-start;
}

.action-buttons :deep(.p-button) {
  padding: 0;
  width: 28px;
  height: 28px;
  margin: 0;
  transition: all 0.2s ease;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.action-buttons :deep(.btn-log) {
  background: #f0f9ff !important;
  border: 1px solid #bae6fd !important;
  color: #0284c7 !important;
}

.action-buttons :deep(.btn-log:hover) {
  background: #e0f2fe !important;
  border-color: #7dd3fc !important;
  transform: translateY(-1px);
}

.action-buttons :deep(.btn-abort) {
  background: #fef2f2 !important;
  border: 1px solid #fecaca !important;
  color: #dc2626 !important;
}

.action-buttons :deep(.btn-abort:hover) {
  background: #fee2e2 !important;
  border-color: #fca5a5 !important;
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(239, 68, 68, 0.1) !important;
}

.event-id {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 13px;
  cursor: pointer;
}

.event-id:hover {
  text-decoration: underline;
}

.text-green { color: #16a34a; }
.text-red { color: #dc2626; }

.target-node {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #0ea5e9;
  background: #f0f9ff;
  padding: 4px 10px 4px 8px;
  border-radius: 4px;
  border: 1px solid #bae6fd;
  max-width: 120px;
}

.target-node span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-node i {
  font-size: 12px;
  color: #0284c7;
}

.time-text {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 11px;
  color: var(--color-text-muted);
}

/* Premium Badges */
.premium-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}

.badge-success { background: #f0fdf4; color: #16a34a; border-color: #dcfce7; }
.badge-failed { background: #fef2f2; color: #dc2626; border-color: #fee2e2; }
.badge-running { background: #eff6ff; color: #2563eb; border-color: #dbeafe; }
.badge-queued, .badge-pending { background: #f5f3ff; color: #7c3aed; border-color: #ede9fe; }
.badge-aborted { background: #f8fafc; color: #64748b; border-color: #e2e8f0; }
</style>
