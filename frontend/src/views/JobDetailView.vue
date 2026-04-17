<script setup lang="ts">
import { onMounted, onUnmounted, ref, inject, type Ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { jobsApi, eventsApi, type Job, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Skeleton from 'primevue/skeleton'
import Breadcrumb from 'primevue/breadcrumb'

const wsStore = useWebSocketStore()
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

const loadData = async () => {
  const id = route.params.id as string
  if (!id) return

  try {
    loading.value = true
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
    message: `确定要触发任务 "${job.value.name}" 吗？`,
    header: '确认触发',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: 'info' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        triggering.value = true
        const result = await jobsApi.trigger(job.value!.id) as unknown as import('@/api').TriggerResponse
        showToast({ severity: 'success', summary: '任务已入队', detail: `Event ID: ${result.event_id}`, life: 5000 })
        loadData()
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

const breadcrumbItems = ref([
  { label: '任务管理', command: () => router.push('/jobs') },
  { label: '任务详情' }
])

const handleTaskStatus = (data: any) => {
  if (job.value && data.job_id === job.value.id) {
    loadData()
  }
}

onMounted(() => {
  loadData()
  wsStore.onMessage('task_status', handleTaskStatus)
  if (globalRefreshHandler) {
    globalRefreshHandler.value = loadData
  }
})

onUnmounted(() => {
  wsStore.offMessage('task_status', handleTaskStatus)
  if (globalRefreshHandler) {
    globalRefreshHandler.value = null
  }
})
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
          <Button severity="info" icon="pi pi-play" :loading="triggering" outlined @click="handleTrigger" label="立即触发" />
          <Button icon="pi pi-pencil" outlined @click="handleEdit" label="编辑" />
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
            <Column field="id" header="Event ID" style="min-width: 180px">
              <template #body="{ data }">
                <span class="link-text event-id" @click="router.push(`/logs/${data.id}`)">{{ data.id.split('_').slice(-1)[0] }}</span>
              </template>
            </Column>
            <Column header="状态" style="width: 110px" alignHeader="center" align="center">
              <template #body="{ data }">
                <span :class="['status-badge', `status-${data.status}`]">
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
                <span class="time-text">{{ data.duration ? `${data.duration}秒` : '-' }}</span>
              </template>
            </Column>
            <Column header="退出码" style="width: 90px" alignHeader="center" align="center">
              <template #body="{ data }">
                <span :class="['exit-code', { 'exit-error': data.exit_code !== 0 && data.exit_code !== undefined && data.exit_code !== null }]">
                  {{ data.exit_code ?? '-' }}
                </span>
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
  gap: 8px;
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
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  font-size: 13px;
  word-break: break-all;
  margin: 0;
  padding: 12px;
  background: #f8fafc;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
}

.time-text {
  font-size: 12px;
  color: var(--color-text-muted);
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
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, monospace;
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

.event-id {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, monospace;
  font-size: 13px;
  cursor: pointer;
}

.event-id:hover {
  text-decoration: underline;
}

.exit-code {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, monospace;
  font-size: 12px;
}

.exit-error {
  color: #dc2626;
  font-weight: 500;
}
</style>
