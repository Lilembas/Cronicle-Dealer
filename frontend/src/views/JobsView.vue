<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { jobsApi, nodesApi, type Node } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Select from 'primevue/select'
import Card from 'primevue/card'
import Paginator from 'primevue/paginator'

const wsStore = useWebSocketStore()
const queryClient = useQueryClient()

const router = useRouter()

const pagination = ref({
  page: 1,
  pageSize: 20,
})

const { data: jobsDataRaw, isLoading, refetch } = useQuery({
  queryKey: ['jobs', pagination],
  queryFn: () => jobsApi.list({
    page: pagination.value.page,
    page_size: pagination.value.pageSize,
  }),
})
const jobsData = jobsDataRaw as unknown as import('vue').Ref<{ total: number; page: number; data: any[] } | undefined>

const selectedGroup = ref('')

const allGroups = computed(() => {
  const jobs = jobsData.value?.data || []
  return Array.from(new Set(jobs.map((job: any) => job.category || '未分组'))).sort()
})

const groupedJobs = computed(() => {
  const jobs = jobsData.value?.data || []
  const map = new Map<string, any[]>()
  jobs.forEach((job: any) => {
    const group = job.category || '未分组'
    if (!map.has(group)) map.set(group, [])
    map.get(group)!.push(job)
  })
  return map
})

const filteredGroups = computed(() => {
  if (!selectedGroup.value) return groupedJobs.value
  const jobs = groupedJobs.value.get(selectedGroup.value)
  return jobs ? new Map([[selectedGroup.value, jobs]]) : new Map()
})

const groupColorMap: Record<string, string> = {}
const groupColors = ['success', 'warn', 'danger', 'info']
let colorIndex = 0

const getGroupColor = (group: string) => {
  if (!groupColorMap[group]) {
    groupColorMap[group] = groupColors[colorIndex % groupColors.length]
    colorIndex++
  }
  return groupColorMap[group]
}

const nodesMap = ref<Map<string, string>>(new Map())

const loadNodes = async () => {
  try {
    const all = await nodesApi.list({}) as unknown as Node[]
    nodesMap.value = new Map((all || []).map(n => [n.id, n.hostname]))
  } catch {
  }
}

const formatTarget = (row: any): { type: 'any' | 'node' | 'tags'; label?: string; tags?: string[] } => {
  const targetType = row.target_type || 'any'
  if (targetType === 'any' || !targetType) return { type: 'any' }
  if (targetType === 'node_id') {
    const hostname = nodesMap.value.get(row.target_value) || row.target_value
    return { type: 'node', label: hostname }
  }
  if (targetType === 'tags') {
    let tags: string[] = []
    try {
      const parsed = JSON.parse(row.target_value)
      if (Array.isArray(parsed)) tags = parsed
    } catch {
      tags = row.target_value ? String(row.target_value).split(',').filter(Boolean) : []
    }
    return { type: 'tags', tags }
  }
  return { type: 'any' }
}

const weekDays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

const formatCron = (expr: string): string => {
  if (!expr) return expr
  const parts = expr.trim().split(/\s+/)
  if (parts.length !== 5) return expr
  const [min, hour, dom, month, dow] = parts

  if (expr === '* * * * *') return '每分钟'
  if (/^\*\/\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    return `每${min.split('/')[1]}分钟`
  }
  if (/^\d+\/\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    const [start, step] = min.split('/')
    return `从第${start}分起每${step}分钟`
  }
  if (min === '0' && /^\*\/\d+$/.test(hour) && dom === '*' && month === '*' && dow === '*') {
    return `每${hour.split('/')[1]}小时`
  }
  if (/^\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    return `每小时第${min}分`
  }
  if (/^\d+$/.test(min) && /^\d+$/.test(hour) && dom === '*' && month === '*' && /^\d+$/.test(dow)) {
    const label = weekDays[parseInt(dow) % 7]
    return `每${label} ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  if (/^\d+$/.test(min) && /^\d+$/.test(hour) && /^\d+$/.test(dom) && month === '*' && dow === '*') {
    return `每月${dom}日 ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  if (/^\d+$/.test(min) && /^\d+$/.test(hour) && dom === '*' && month === '*' && dow === '*') {
    return `每天 ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  return expr
}

const handleCreate = () => {
  router.push('/jobs/new')
}

const handleEdit = (id: string) => {
  router.push(`/jobs/${id}`)
}

const handleDetail = (id: string) => {
  router.push(`/jobs/${id}/detail`)
}

const handleHistory = (id: string) => {
  router.push(`/jobs/${id}/history`)
}

const handleDelete = async (id: string, name: string) => {
  showConfirm({
    message: `确定要删除任务 "${name}" 吗？`,
    header: '确认删除',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: 'danger' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        await jobsApi.delete(id)
        showToast({ severity: 'success', summary: '删除成功', life: 3000 })
        refetch()
      } catch {
        showToast({ severity: 'error', summary: '删除失败', life: 5000 })
      }
    },
  })
}

const handleTrigger = async (id: string, name: string) => {
  showConfirm({
    message: `确定要触发任务 "${name}" 吗？`,
    header: '确认触发',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: 'info' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        const result = await jobsApi.trigger(id) as unknown as import('@/api').TriggerResponse
        showToast({
          severity: 'success',
          summary: '任务已入队',
          detail: `任务 "${name}" 已入队，Event ID: ${result.event_id}`,
          life: 5000
        })
      } catch (error: any) {
        showToast({ severity: 'error', summary: '触发失败', detail: error.response?.data?.error || '触发失败', life: 5000 })
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

const handleTaskStatus = async () => {
  queryClient.invalidateQueries({ queryKey: ['jobs'] })
  await nextTick()
}

onMounted(() => {
  wsStore.onMessage('task_status', handleTaskStatus)
  loadNodes()
})

onUnmounted(() => {
  wsStore.offMessage('task_status', handleTaskStatus)
})

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
</script>

<template>
  <div class="jobs">
    <div class="page-header">
      <div class="header-actions">
        <Button icon="pi pi-refresh" text @click="() => refetch()" label="刷新" />
        <Button severity="info" icon="pi pi-plus" @click="handleCreate" label="新建任务" />
      </div>
      <Select
        v-model="selectedGroup"
        :options="allGroups"
        placeholder="全部分组"
        showClear
        class="w-40"
      />
    </div>

    <Card>
      <template #content>
        <div v-if="isLoading" class="flex justify-center py-8">
          <i class="pi pi-spin pi-spinner text-2xl text-gray-400"></i>
        </div>

        <template v-else>
          <!-- 按分组渲染 -->
          <div v-for="[group, jobs] in filteredGroups" :key="group" class="group-section">
            <div class="group-header">
              <Tag :value="group" :severity="getGroupColor(group)" />
              <span class="group-count">{{ jobs.length }} 个任务</span>
            </div>
            <DataTable :value="jobs" stripedRows size="small" class="group-table">
              <Column field="name" header="任务名称" style="min-width: 120px">
                <template #body="{ data }">
                  <span class="link-text" @click="handleDetail(data.id)">{{ data.name }}</span>
                </template>
              </Column>
              <Column header="执行节点" style="width: 150px">
                <template #body="{ data }">
                  <template v-if="formatTarget(data).type === 'any'">
                    <Tag value="任意节点" severity="secondary" />
                  </template>
                  <template v-else-if="formatTarget(data).type === 'node'">
                    <Tag :value="formatTarget(data).label" severity="info" class="max-w-[130px] truncate" />
                  </template>
                  <template v-else-if="formatTarget(data).type === 'tags'">
                    <div class="target-tags">
                      <Tag
                        v-for="tag in formatTarget(data).tags"
                        :key="tag"
                        :value="tag"
                        severity="warn"
                      />
                    </div>
                  </template>
                </template>
              </Column>
              <Column header="状态" style="width: 90px" alignHeader="center" align="center">
                <template #body="{ data }">
                  <Tag :value="data.enabled ? '启用' : '禁用'" :severity="data.enabled ? 'success' : 'secondary'" />
                </template>
              </Column>
              <Column header="最后执行" style="width: 100px" alignHeader="center" align="center">
                <template #body="{ data }">
                  <span v-if="data.last_status && data.last_status !== '-'" :class="['status-badge', `status-${data.last_status}`]">
                    <i v-if="data.last_status === 'success'" class="pi pi-check-circle"></i>
                    <i v-else-if="data.last_status === 'failed'" class="pi pi-times-circle"></i>
                    <i v-else-if="data.last_status === 'running'" class="pi pi-spin pi-spinner"></i>
                    <i v-else class="pi pi-clock"></i>
                    <span>{{ getStatusText(data.last_status) }}</span>
                  </span>
                  <span v-else>-</span>
                </template>
              </Column>
              <Column field="cron_expr" header="执行计划" style="width: 160px">
                <template #body="{ data }">
                  <span v-tooltip.top="data.cron_expr" class="cron-label">{{ formatCron(data.cron_expr) }}</span>
                </template>
              </Column>
              <Column field="next_run_time" header="下次执行" style="width: 170px">
                <template #body="{ data }">
                  {{ data.next_run_time ? new Date(data.next_run_time).toLocaleString('zh-CN') : '-' }}
                </template>
              </Column>
              <Column header="操作" frozen alignFrozen="right" style="width: 160px">
                <template #body="{ data }">
                  <div class="action-row">
                    <Button v-tooltip.top="'详情'" size="small" icon="pi pi-eye" text rounded @click="handleDetail(data.id)" />
                    <Button v-tooltip.top="'历史'" size="small" icon="pi pi-clock" text rounded @click="handleHistory(data.id)" />
                    <Button v-tooltip.top="'触发'" size="small" icon="pi pi-play" severity="info" text rounded @click="handleTrigger(data.id, data.name)" />
                    <Button v-tooltip.top="'编辑'" size="small" icon="pi pi-pencil" text rounded @click="handleEdit(data.id)" />
                    <Button v-tooltip.top="'删除'" size="small" icon="pi pi-trash" severity="danger" text rounded @click="handleDelete(data.id, data.name)" />
                  </div>
                </template>
              </Column>
            </DataTable>
          </div>

          <!-- 分页 -->
          <div v-if="jobsData && jobsData.total > 0" class="pagination">
            <Paginator
              v-model:first="paginatorFirst"
              :rows="pagination.pageSize"
              :totalRecords="jobsData.total"
              :rowsPerPageOptions="[10, 20, 50, 100]"
              @page="onPageChange"
              template="FirstPageLink PrevPageLink PageLinks NextPageLink LastPageLink RowsPerPageDropdown CurrentPageReport"
              currentPageReportTemplate="第 {first} 到 {last} 条，共 {totalRecords} 条"
            />
          </div>

          <div v-if="!isLoading && (!jobsData?.data || jobsData.data.length === 0)" class="text-center py-8 text-gray-400">
            <i class="pi pi-inbox text-4xl mb-2 block"></i>
            <p>暂无任务数据</p>
          </div>
        </template>
      </template>
    </Card>
  </div>
</template>

<style scoped>
.jobs {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.group-section {
  margin-bottom: 20px;
}

.group-section:last-of-type {
  margin-bottom: 0;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  padding-left: 2px;
}

.group-count {
  font-size: 12px;
  color: var(--color-text-muted);
}

.cron-label {
  font-size: 13px;
  color: var(--color-text-muted);
  cursor: default;
}

.target-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.action-row {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-wrap: nowrap;
}

.action-row :deep(.p-button) {
  padding: 4px;
  margin: 0;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}
</style>
