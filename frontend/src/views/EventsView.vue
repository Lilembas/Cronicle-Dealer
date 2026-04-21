<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, inject, watch, type Ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { eventsApi, jobsApi, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { useSystemStore } from '@/stores/system'
import { showToast } from '@/utils/toast'
import { showConfirm, hl } from '@/utils/confirm'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Paginator from 'primevue/paginator'
import ProgressBar from 'primevue/progressbar'

const wsStore = useWebSocketStore()
const systemStore = useSystemStore()
const queryClient = useQueryClient()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')


const router = useRouter()
const route = useRoute()

const highlightId = ref(route.query.highlight as string || '')

const filters = ref({
  status: '',
  jobName: '',
  jobCategory: '',
  startDate: '',
  endDate: '',
})

const pagination = ref({
  page: 1,
  pageSize: 20,
})

const { data: eventsDataRaw, isLoading, refetch } = useQuery({
  queryKey: ['events', pagination, filters],
  queryFn: () => eventsApi.list({
    page: pagination.value.page,
    page_size: pagination.value.pageSize,
    status: filters.value.status || undefined,
    job_category: filters.value.jobCategory || undefined,
  }),
})
const eventsData = eventsDataRaw as unknown as { total: number; data: Event[] } | undefined

onMounted(async () => {
  if (highlightId.value) {
    nextTick(() => {
      const el = document.getElementById(`event-row-${highlightId.value}`)
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    })
  }
  if (globalRefreshHandler) {
    globalRefreshHandler.value = () => refetch()
  }
})

onUnmounted(() => {
})

const statusOptions = [
  { label: '全部', value: '' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '运行中', value: 'running' },
  { label: '已入队', value: 'queued' },
]

const categoryOptions = ref<{ label: string; value: string }[]>([])
const loadCategories = async () => {
  const { data } = await jobsApi.list({ page_size: 1000 }) as unknown as { data: any[] }
  const categories = [...new Set((data || []).map((j: any) => j.category).filter(Boolean))]
  categoryOptions.value = [
    { label: '全部', value: '' },
    ...categories.map((c: string) => ({ label: c, value: c })),
  ]
}

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

const formatBytes = (bytes: number) => {
  const gb = bytes / (1024 * 1024 * 1024)
  return `${gb.toFixed(2)} GB`
}

const viewDetail = (event: Event) => {
  router.push(`/logs/${event.id}`)
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
        refetch()
      } catch {
        showToast({ severity: 'error', summary: '中止失败', life: 5000 })
      }
    },
  })
}

const resetFilter = () => {
  filters.value = {
    status: '',
    jobName: '',
    jobCategory: '',
    startDate: '',
    endDate: '',
  }
}

watch(filters, () => {
  pagination.value.page = 1
}, { deep: true })

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

const getRowClass = (data: Event) => {
  return data.id === highlightId.value ? 'row-highlight' : ''
}

const handleTaskStatus = () => {
  queryClient.invalidateQueries({ queryKey: ['events'] })
}

onMounted(() => {
  loadCategories()
  wsStore.onMessage('task_status', handleTaskStatus)
})

onUnmounted(() => {
  wsStore.offMessage('task_status', handleTaskStatus)
})
</script>

<template>
  <div class="events">
    <!-- 筛选栏 -->
    <Card class="filter-card">
      <template #content>
        <div class="flex flex-wrap items-end gap-4">
          <div class="flex flex-col gap-1">
            <label class="font-medium text-sm">状态</label>
            <Select v-model="filters.status" :options="statusOptions" optionLabel="label" optionValue="value" placeholder="选择状态" class="w-36" showClear />
          </div>

          <div class="flex flex-col gap-1">
            <label class="font-medium text-sm">任务名称</label>
            <InputText v-model="filters.jobName" placeholder="输入任务名称" class="w-48" />
          </div>

          <div class="flex flex-col gap-1">
            <label class="font-medium text-sm">任务分组</label>
            <Select v-model="filters.jobCategory" :options="categoryOptions" optionLabel="label" optionValue="value" placeholder="选择分组" class="w-36" showClear />
          </div>

          <Button severity="secondary" @click="resetFilter" label="重置" size="small" outlined />
        </div>
      </template>
    </Card>

    <!-- 执行记录列表 -->
    <Card class="table-card">
      <template #content>
        <DataTable
          :value="eventsData?.data || []"
          :loading="isLoading"
          stripedRows
          class="events-table"
          :rowClass="getRowClass"
          rowHover
        >
          <Column field="id" header="Event ID" style="width: 100px">
            <template #body="{ data }">
              <span
                v-tooltip.top="data.id"
                class="event-id link-text"
                :id="`event-row-${data.id}`"
                @click="viewDetail(data)"
              >{{ data.id.split('_').slice(-1)[0] }}</span>
            </template>
          </Column>

          <Column field="job_name" header="任务名称" style="width: 180px">
            <template #body="{ data }">
              <span v-if="data.job_id" class="job-name-link" @click="router.push(`/jobs/${data.job_id}/detail`)">{{ data.job_name || data.job_id }}</span>
              <span v-else>{{ data.job_name || data.job_id }}</span>
            </template>
          </Column>

          <Column header="执行节点" style="width: 140px">
            <template #body="{ data }">
              <span v-if="data.node_name" class="target-node" :title="data.node_name">
                <i class="pi pi-desktop" />
                <span class="node-name-text">{{ data.node_name }}</span>
              </span>
              <span v-else class="text-gray-400">-</span>
            </template>
          </Column>

          <Column header="分组" style="width: 110px">
            <template #body="{ data }">
              <Tag v-if="data.job_category" :value="data.job_category" severity="info" :pt="{ 
                root: { style: { height: '20px', padding: '0 6px' } },
                label: { style: { fontWeight: 'normal', fontSize: '10px' } } 
              }" />
              <span v-else class="text-gray-400 text-xs">-</span>
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

          <Column header="执行时间" style="width: 200px">
            <template #body="{ data }">
              <div v-if="data.start_time" class="time-text">
                <div>{{ new Date(data.start_time).toLocaleString('zh-CN') }}</div>
                <div v-if="data.end_time" class="time-text-end">
                  至 {{ new Date(data.end_time).toLocaleString('zh-CN') }}
                </div>
              </div>
              <span v-else>-</span>
            </template>
          </Column>

          <Column header="持续时长" style="width: 140px" align="right">
            <template #body="{ data }">
              <span v-if="data.status === 'running' && data.start_time" class="time-text font-mono text-blue-500 font-bold">
                {{ formatDuration(Math.max(0, Math.floor((systemStore.currentTime - new Date(data.start_time).getTime()) / 1000))) }}
              </span>
              <span v-else-if="data.duration" class="time-text font-mono">{{ formatDuration(data.duration) }}</span>
              <span v-else class="time-text font-mono text-gray-400">-</span>
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

          <Column field="cpu_percent" header="CPU" style="width: 100px" alignHeader="center">
            <template #body="{ data }">
              <div class="cpu-metric" v-if="data.cpu_percent !== undefined && data.cpu_percent !== null">
                <ProgressBar :value="Math.min(data.cpu_percent, 100)" :showValue="false" class="mini-progress" />
                <span class="metric-text">{{ data.cpu_percent.toFixed(1) }}%</span>
              </div>
              <span v-else>-</span>
            </template>
          </Column>

          <Column field="memory_bytes" header="内存" style="width: 100px" alignHeader="center">
            <template #body="{ data }">
              <span v-if="data.memory_bytes != null" class="metric-text">{{ formatBytes(data.memory_bytes) }}</span>
              <span v-else>-</span>
            </template>
          </Column>

          <Column header="操作" frozen alignFrozen="right" style="width: 100px">
            <template #body="{ data }">
              <div class="action-buttons">
                <Button v-tooltip.top="'日志'" size="small" icon="pi pi-eye" severity="info" class="btn-log" @click="viewDetail(data)" />
                <Button v-if="canAbort(data.status)" v-tooltip.top="'中止'" size="small" icon="pi pi-stop-circle" severity="danger" class="btn-abort" @click="handleAbort(data)" />
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
.events {
  padding: 16px 24px 24px 24px;
  max-width: 1500px;
  margin: 0 auto;
}

.filter-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
  margin-bottom: 16px;
}

.table-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
}

.events-table {
  width: 100%;
}

.event-id {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 13px;
}

.events-table :deep(.p-datatable-tbody > tr > td) {
  padding: 8px 12px;
  font-size: 11px;
}

.job-name-link {
  color: var(--color-brand);
  cursor: pointer;
  font-weight: 500;
  font-size: 12px;
}

.job-name-link:hover {
  text-decoration: underline;
}

:deep(.row-highlight) {
  background-color: #fef9c3 !important;
  transition: background-color 2s ease;
}

.text-sm {
  font-size: 12px;
}

.text-green {
  color: #10b981;
  font-weight: 500;
}

.text-red {
  color: #ef4444;
  font-weight: 500;
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

.time-text {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 11px;
  color: var(--color-text-muted);
}

.time-text-end {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 11px;
  color: var(--color-text-muted);
  opacity: 0.8;
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

.badge-success {
  background: #f0fdf4;
  color: #16a34a;
  border-color: #dcfce7;
}

.badge-failed {
  background: #fef2f2;
  color: #dc2626;
  border-color: #fee2e2;
}

.badge-running {
  background: #eff6ff;
  color: #2563eb;
  border-color: #dbeafe;
}

.badge-queued, .badge-pending {
  background: #f5f3ff;
  color: #7c3aed;
  border-color: #ede9fe;
}

.badge-aborted {
  background: #f8fafc;
  color: #64748b;
  border-color: #e2e8f0;
}

.cpu-metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}

.mini-progress {
  height: 4px !important;
  background: #f1f5f9 !important;
}

.mini-progress :deep(.p-progressbar-value) {
  background: #0ea5e9;
  border-radius: 2px;
}

.metric-text {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 11px;
  color: var(--color-text-secondary);
}

.target-node {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: #0284c7;
  background: #f0f9ff;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid #bae6fd;
  max-width: 140px;
}

.node-name-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.target-node i {
  font-size: 12px;
  color: #0284c7;
}

.text-green { color: #16a34a; }
.text-red { color: #dc2626; }

@media (max-width: 768px) {
  .events {
    padding: 16px;
  }

  .events-table :deep(.p-datatable-wrapper) {
    overflow-x: auto;
  }
}
</style>
