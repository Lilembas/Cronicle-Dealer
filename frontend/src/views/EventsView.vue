<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { eventsApi, jobsApi, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Paginator from 'primevue/paginator'

const wsStore = useWebSocketStore()
const queryClient = useQueryClient()


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
    job_name: filters.value.jobName || undefined,
    job_category: filters.value.jobCategory || undefined,
  }),
})
const eventsData = eventsDataRaw as unknown as { total: number; data: Event[] } | undefined

onMounted(() => {
  if (highlightId.value) {
    nextTick(() => {
      const el = document.getElementById(`event-row-${highlightId.value}`)
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
      }
    })
  }
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

const viewDetail = (event: Event) => {
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
        refetch()
      } catch {
        showToast({ severity: 'error', summary: '中止失败', life: 5000 })
      }
    },
  })
}

const applyFilter = () => {
  pagination.value.page = 1
  refetch()
}

const resetFilter = () => {
  filters.value = {
    status: '',
    jobName: '',
    jobCategory: '',
    startDate: '',
    endDate: '',
  }
  pagination.value.page = 1
  refetch()
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

const getRowClass = (data: Event) => {
  return data.id === highlightId ? 'row-highlight' : ''
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
    <div class="page-header">
      <Button icon="pi pi-refresh" text @click="() => refetch()" label="刷新" />
    </div>

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

          <Button severity="info" icon="pi pi-filter" @click="applyFilter" label="筛选" />
          <Button severity="secondary" @click="resetFilter" label="重置" />
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
          <Column field="id" header="Event ID" style="min-width: 180px">
            <template #body="{ data }">
              <span
                v-tooltip.top="data.id"
                class="event-id link-text"
                :id="`event-row-${data.id}`"
                @click="viewDetail(data)"
              >{{ data.id.split('_').slice(-1)[0] }}</span>
            </template>
          </Column>

          <Column field="job_name" header="任务名称" style="min-width: 150px">
            <template #body="{ data }">
              <span v-if="data.job_id" class="text-primary-500 cursor-pointer hover:underline" @click="router.push(`/jobs/${data.job_id}/detail`)">{{ data.job_name || data.job_id }}</span>
              <span v-else>{{ data.job_name || data.job_id }}</span>
            </template>
          </Column>

          <Column header="分组" style="width: 120px">
            <template #body="{ data }">
              <Tag v-if="data.job_category" :value="data.job_category" severity="info" />
              <span v-else class="text-gray-400">-</span>
            </template>
          </Column>

          <Column header="状态" style="width: 100px" alignHeader="center" align="center">
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

          <Column header="执行时间" style="width: 180px">
            <template #body="{ data }">
              <div v-if="data.start_time">
                <div>{{ new Date(data.start_time).toLocaleString('zh-CN') }}</div>
                <div v-if="data.end_time" class="text-sm text-gray-400">
                  至 {{ new Date(data.end_time).toLocaleString('zh-CN') }}
                </div>
              </div>
              <span v-else>-</span>
            </template>
          </Column>

          <Column header="持续时间" style="width: 120px">
            <template #body="{ data }">
              {{ formatDuration(data.duration) }}
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

          <Column field="cpu_percent" header="CPU" style="width: 100px" alignHeader="right" align="right">
            <template #body="{ data }">
              <span v-if="data.cpu_percent !== undefined">{{ data.cpu_percent.toFixed(1) }}%</span>
              <span v-else>-</span>
            </template>
          </Column>

          <Column header="操作" frozen alignFrozen="right" style="width: 180px">
            <template #body="{ data }">
              <div class="flex gap-2">
                <Button severity="info" size="small" icon="pi pi-eye" @click="viewDetail(data)" label="日志" />
                <Button v-if="canAbort(data.status)" severity="danger" size="small" icon="pi pi-stop-circle" @click="handleAbort(data)" label="中止" />
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
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  margin-bottom: 20px;
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
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  font-size: 13px;
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

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 768px) {
  .events {
    padding: 16px;
  }

  .events-table :deep(.p-datatable-wrapper) {
    overflow-x: auto;
  }
}
</style>
