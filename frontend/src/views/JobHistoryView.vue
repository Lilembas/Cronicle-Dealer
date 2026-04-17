<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, inject, type Ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { eventsApi, jobsApi, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { useSystemStore } from '@/stores/system'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Paginator from 'primevue/paginator'
import Breadcrumb from 'primevue/breadcrumb'
import { type Job } from '@/api'

const wsStore = useWebSocketStore()
const systemStore = useSystemStore()
const queryClient = useQueryClient()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')

const router = useRouter()
const route = useRoute()

const jobId = route.params.id as string

const { data: jobDataRaw } = useQuery({
  queryKey: ['job', jobId],
  queryFn: () => jobsApi.get(jobId),
})
const jobData = computed(() => (jobDataRaw.value as any)?.data as Job | undefined)

const pagination = ref({
  page: 1,
  pageSize: 20,
})

const { data: eventsDataRaw, isLoading, refetch } = useQuery({
  queryKey: ['job-events', jobId, pagination],
  queryFn: () => eventsApi.list({
    page: pagination.value.page,
    page_size: pagination.value.pageSize,
    job_id: jobId,
  }),
})
const eventsData = eventsDataRaw as unknown as { total: number; data: Event[] } | undefined

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

const formatDuration = (seconds: number) => {
  if (!seconds) return '-'
  if (seconds < 60) return `${seconds.toFixed(2)}秒`
  if (seconds < 3600) return `${(seconds / 60).toFixed(2)}分钟`
  return `${(seconds / 3600).toFixed(2)}小时`
}

const breadcrumbItems = computed(() => [
  { label: '任务管理', command: () => router.push('/jobs') },
  { label: '执行历史' }
])

// Removed goBack as it is unused

const viewLog = (event: Event) => {
  router.push(`/logs/${event.id}`)
}

const canAbort = (status: string) => status === 'running' || status === 'pending' || status === 'queued'

const handleAbort = async (event: Event) => {
  showConfirm({
    message: `确认中止任务 ${event.id} 吗？`,
    header: '中止确认',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确认', severity: 'danger' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        await eventsApi.abort(event.id)
        showToast({ severity: 'success', summary: '中止请求已提交', life: 3000 })
        queryClient.invalidateQueries({ queryKey: ['events'] })
      } catch {
        showToast({ severity: 'error', summary: '中止失败', life: 5000 })
      }
    },
  })
}

const paginatorFirst = computed({
  get: () => (pagination.value.page - 1) * pagination.value.pageSize,
  set: (val: number) => {
    pagination.value.page = Math.floor(val / pagination.value.pageSize) + 1
  }
})

const onPageChange = (event: any) => {
  pagination.value.page = Math.floor(event.first / event.rows) + 1
  pagination.value.pageSize = event.rows
}

const handleTaskStatus = (data: any) => {
  if (data.job_id === jobId) {
    queryClient.invalidateQueries({ queryKey: ['job-events', jobId] })
  }
}

onMounted(() => {
  wsStore.onMessage('task_status', handleTaskStatus)
  if (globalRefreshHandler) {
    globalRefreshHandler.value = () => refetch()
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
  <div class="job-history">
    <div class="page-header">
      <div class="header-left">
        <Breadcrumb :model="breadcrumbItems" />
        <span v-if="jobData?.name" class="job-name-display">
          <i class="pi pi-briefcase"></i>
          {{ jobData.name }}
        </span>
      </div>
          </div>

    <Card class="table-card">
      <template #content>
        <DataTable
          :value="eventsData?.data || []"
          :loading="isLoading"
          stripedRows
          class="events-table"
        >
          <Column field="id" header="Event ID" style="width: 140px">
            <template #body="{ data }">
              <span class="link-text event-id" @click="viewLog(data)">{{ data.id.split('_').slice(-1)[0] }}</span>
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

          <Column header="状态" style="width: 120px" alignHeader="center" align="center">
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

          <Column header="持续时长" style="width: 120px">
            <template #body="{ data }">
              <span v-if="data.status === 'running' && data.start_time" class="time-text text-blue-500 font-bold">
                {{ formatDuration(Math.max(0, Math.floor((systemStore.currentTime - new Date(data.start_time).getTime()) / 1000))) }}
              </span>
              <span v-else class="time-text">{{ formatDuration(data.duration) }}</span>
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
                <Button v-tooltip.top="'日志'" size="small" icon="pi pi-eye" class="action-btn" :pt="{ 
                  root: { style: { background: '#f0f9ff', borderColor: '#bae6fd', color: '#0284c7' } } 
                }" @click="viewLog(data)" />
                <Button v-if="canAbort(data.status)" v-tooltip.top="'中止'" size="small" icon="pi pi-stop-circle" class="action-btn" :pt="{ 
                  root: { style: { background: '#fef2f2', borderColor: '#fecaca', color: '#dc2626' } } 
                }" @click="handleAbort(data)" />
              </div>
            </template>
          </Column>
        </DataTable>

        <div v-if="eventsData && eventsData.total > 0" class="pagination">
          <Paginator
            v-model:first="paginatorFirst"
            :rows="pagination.pageSize"
            :totalRecords="eventsData.total"
            :rowsPerPageOptions="[10, 20, 50, 100]"
            @page="onPageChange"
            template="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink RowsPerPageDropdown CurrentPageReport"
            currentPageReportTemplate="第 {first} 到 {last} 条，共 {totalRecords} 条"
          />
        </div>

        <div v-if="!isLoading && (!eventsData?.data || (eventsData?.data?.length || 0) === 0)" class="text-center py-8 text-gray-400">
          <i class="pi pi-inbox text-4xl mb-2 block"></i>
          <p>暂无执行记录</p>
        </div>
      </template>
    </Card>
  </div>
</template>

<style scoped>
.job-history {
  padding: 24px;
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.job-name-display {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--color-text-primary);
  font-weight: 500;
  padding: 4px 12px;
  background: #f0f9ff;
  border-radius: 6px;
  border: 1px solid #bae6fd;
}

.job-name-display i {
  font-size: 14px;
  color: #0284c7;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.table-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
}

.events-table {
  width: 100%;
}

.action-buttons {
  display: flex;
  gap: 6px;
  justify-content: flex-start;
}

.action-btn {
  width: 28px !important;
  height: 28px !important;
  padding: 0 !important;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1) !important;
}

.action-btn:hover {
  transform: translateY(-1px);
  filter: brightness(0.95);
}

.event-id {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  font-size: 13px;
}

.text-green { color: #16a34a; font-family: 'Fira Code', monospace; }
.text-red { color: #dc2626; font-family: 'Fira Code', monospace; }

.time-text {
  font-family: 'JetBrains Mono', 'Fira Code', ui-monospace, SFMono-Regular, monospace;
  font-size: 11px;
  color: var(--color-text-muted);
}

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
