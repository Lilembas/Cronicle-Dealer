<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, inject, type Ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { eventsApi, jobsApi, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import Button from 'primevue/button'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Paginator from 'primevue/paginator'
import Breadcrumb from 'primevue/breadcrumb'

const wsStore = useWebSocketStore()
const queryClient = useQueryClient()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')

const router = useRouter()
const route = useRoute()

const jobId = route.params.id as string

const { data: jobData } = useQuery({
  queryKey: ['job', jobId],
  queryFn: () => jobsApi.get(jobId),
})

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

const goBack = () => {
  router.push('/jobs')
}

const viewLog = (event: Event) => {
  router.push(`/logs/${event.id}`)
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
          <Column field="id" header="Event ID" style="min-width: 180px">
            <template #body="{ data }">
              <span class="link-text event-id" @click="viewLog(data)">{{ data.id.split('_').slice(-1)[0] }}</span>
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

          <Column header="开始时间" style="width: 180px">
            <template #body="{ data }">
              <span class="time-text">{{ data.start_time ? new Date(data.start_time).toLocaleString('zh-CN') : '-' }}</span>
            </template>
          </Column>

          <Column header="持续时间" style="width: 120px">
            <template #body="{ data }">
              <span class="time-text">{{ formatDuration(data.duration) }}</span>
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

          <Column header="执行节点" style="width: 120px">
            <template #body="{ data }">
              <span v-if="data.node_name" class="target-node">
                <i class="pi pi-desktop" />
                <span>{{ data.node_name }}</span>
              </span>
              <span v-else>-</span>
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

.event-id {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  font-size: 13px;
}

.text-green {
  color: #10b981;
  font-weight: 500;
}

.text-red {
  color: #ef4444;
  font-weight: 500;
}

.time-text {
  font-size: 12px;
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

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
