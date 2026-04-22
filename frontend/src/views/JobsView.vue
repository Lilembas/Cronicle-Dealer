<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, inject, type Ref } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { jobsApi, nodesApi, type Node } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import { showConfirm, hl } from '@/utils/confirm'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import Badge from 'primevue/badge'
import Select from 'primevue/select'
import Card from 'primevue/card'
import Paginator from 'primevue/paginator'

const wsStore = useWebSocketStore()
const authStore = useAuthStore()
const queryClient = useQueryClient()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')

const router = useRouter()

const canManage = computed(() => authStore.isAdmin || authStore.user?.role === 'user')

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

const getGroupIcon = (_group: string) => 'pi pi-server'

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

  // Handle 5-field (legacy) expressions
  if (parts.length === 5) {
    const [min, hour, dom, month, dow] = parts
    if (expr === '* * * * *') return '每分钟'
    if (/^\*\/\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
      return `每${min.split('/')[1]}分钟`
    }
    if (/^\d+$/.test(min) && /^\d+$/.test(hour) && dom === '*' && month === '*' && dow === '*') {
      return `每天 ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
    }
    return expr
  }

  // Handle 6-field (with seconds) expressions
  if (parts.length !== 6) return expr
  const [sec, min, hour, dom, month, dow] = parts

  if (expr === '* * * * * *') return '每秒'
  if (/^\*\/\d+$/.test(sec) && min === '*' && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    return `每${sec.split('/')[1]}秒`
  }
  if (sec === '0' && expr === '0 * * * * *') return '每分钟'
  if (sec === '0' && /^\*\/\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    return `每${min.split('/')[1]}分钟`
  }
  if (sec === '0' && /^\d+\/\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    const [start, step] = min.split('/')
    return `从第${start}分起每${step}分钟`
  }
  if (sec === '0' && min === '0' && /^\*\/\d+$/.test(hour) && dom === '*' && month === '*' && dow === '*') {
    return `每${hour.split('/')[1]}小时`
  }
  if (sec === '0' && /^\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    return `每小时第${min}分`
  }
  if (sec === '0' && /^\d+$/.test(min) && /^\d+$/.test(hour) && dom === '*' && month === '*' && /^\d+$/.test(dow)) {
    const label = weekDays[parseInt(dow) % 7]
    return `每${label} ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  if (sec === '0' && /^\d+$/.test(min) && /^\d+$/.test(hour) && /^\d+$/.test(dom) && month === '*' && dow === '*') {
    return `每月${dom}日 ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  if (sec === '0' && /^\d+$/.test(min) && /^\d+$/.test(hour) && dom === '*' && month === '*' && dow === '*') {
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
  if (!canManage.value) return
  showConfirm({
    message: `确定要删除任务 ${hl(name)} 吗？`,
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

const toggleJobEnabled = async (job: any) => {
  if (!canManage.value) return
  const newEnabled = !job.enabled
  const action = newEnabled ? '启用' : '禁用'
  showConfirm({
    message: `确定要${action}任务 ${hl(job.name)} 吗？`,
    header: `确认${action}`,
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: newEnabled ? 'success' : 'warn' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        await jobsApi.update(job.id, { enabled: newEnabled })
        showToast({ severity: 'success', summary: `${action}成功`, detail: `任务 "${job.name}" 已${action}`, life: 3000 })
        refetch()
      } catch (error: any) {
        showToast({ severity: 'error', summary: `${action}失败`, detail: error.response?.data?.error || `${action}失败`, life: 5000 })
      }
    },
  })
}

const handleTrigger = async (id: string, name: string) => {
  if (!canManage.value) return
  showConfirm({
    message: `确定要触发任务 ${hl(name)} 吗？`,
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
          detail: `任务 ${name} 已入队，Event ID: ${result.event_id}`,
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
  <div class="jobs page-container">
    <Card>
      <template #header>
        <div class="card-header">
          <Select
            v-model="selectedGroup"
            :options="allGroups"
            placeholder="全部分组"
            showClear
            class="group-select"
            size="small"
            :pt="{
              root: { style: { width: '100px', height: '24px' } },
              label: { style: { fontSize: '11px', padding: '0 8px', display: 'flex', alignItems: 'center' } },
              option: { style: { padding: '2px 8px', fontSize: '10px' } }
            }"
          />
        </div>
      </template>
      <template #content>
        <div v-if="isLoading" class="flex justify-center py-8">
          <i class="pi pi-spin pi-spinner text-2xl text-gray-400"></i>
        </div>

        <template v-else>
          <!-- 按分组渲染 -->
          <div v-for="[group, jobs] in filteredGroups" :key="group" class="group-section">
            <div class="group-header">
              <i :class="getGroupIcon(group)" class="group-icon" />
              <span class="group-name">{{ group }}</span>
              <Badge :value="jobs.length" severity="secondary" />
            </div>
            <DataTable :value="jobs" stripedRows size="small" class="group-table">
              <Column field="name" header="任务名称" style="min-width: 120px">
                <template #body="{ data }">
                  <span class="link-text" @click="handleDetail(data.id)">{{ data.name }}</span>
                </template>
              </Column>
              <Column header="执行节点" style="width: 180px" alignHeader="center">
                <template #body="{ data }">
                  <template v-if="formatTarget(data).type === 'any'">
                    <span class="target-any">
                      <i class="pi pi-arrows-alt" />
                      <span>任意节点</span>
                    </span>
                  </template>
                  <template v-else-if="formatTarget(data).type === 'node'">
                    <span class="target-node">
                      <i class="pi pi-desktop" />
                      <span>{{ formatTarget(data).label }}</span>
                    </span>
                  </template>
                  <template v-else-if="formatTarget(data).type === 'tags'">
                    <div class="target-node-list">
                      <span
                        v-for="tag in formatTarget(data).tags"
                        :key="tag"
                        class="target-tag"
                      >
                        <i class="pi pi-sitemap" />
                        <span>{{ tag }}</span>
                      </span>
                    </div>
                  </template>
                </template>
              </Column>
              <Column header="开启" style="width: 90px" alignHeader="center" align="center">
                <template #body="{ data }">
                  <Tag
                    :value="data.enabled ? '启用' : '禁用'"
                    :severity="data.enabled ? 'success' : 'secondary'"
                    class="select-none"
                    :class="{ 'cursor-pointer': canManage }"
                    v-tooltip.top="canManage ? `点击${data.enabled ? '禁用' : '启用'}此任务` : '无操作权限'"
                    @click="toggleJobEnabled(data)"
                  />
                </template>
              </Column>
              <Column header="状态" style="width: 100px" alignHeader="center" align="center">
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
                  <span class="time-text">{{ (data.enabled && data.next_run_time) ? new Date(data.next_run_time).toLocaleString('zh-CN') : '-' }}</span>
                </template>
              </Column>
              <Column header="操作" frozen alignFrozen="right" alignHeader="center" style="width: 160px">
                <template #body="{ data }">
                  <div class="action-row">
                    <Button v-tooltip.top="'详情'" size="small" icon="pi pi-eye" outlined severity="secondary" class="action-btn" @click="handleDetail(data.id)" />
                    <Button v-tooltip.top="'历史'" size="small" icon="pi pi-clock" outlined severity="secondary" class="action-btn" @click="handleHistory(data.id)" />
                    <Button v-tooltip.top="canManage ? '触发' : '无操作权限'" size="small" icon="pi pi-play" class="action-btn btn-trigger" :disabled="!canManage" @click="handleTrigger(data.id, data.name)" />
                    <Button v-tooltip.top="canManage ? '编辑' : '无操作权限'" size="small" icon="pi pi-pencil" outlined severity="secondary" class="action-btn" :disabled="!canManage" @click="handleEdit(data.id)" />
                    <Button v-tooltip.top="canManage ? '删除' : '无操作权限'" size="small" icon="pi pi-trash" class="action-btn btn-delete" :disabled="!canManage" @click="handleDelete(data.id, data.name)" />
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
              :pt="{
                root: { style: { fontSize: '11px', gap: '2px', padding: '0' } },
                page: { style: { minWidth: '26px', height: '26px', fontSize: '11px' } },
                previous: { style: { width: '26px', height: '26px' } },
                next: { style: { width: '26px', height: '26px' } },
                first: { style: { width: '26px', height: '26px' } },
                last: { style: { width: '26px', height: '26px' } },
                current: { style: { fontSize: '11px', height: '26px', justifySelf: 'center', display: 'flex', alignItems: 'center' } },
                pcRowsPerPageDropdown: {
                  root: { style: { height: '24px', fontSize: '11px', minWidth: '54px' } },
                  label: { style: { padding: '0 4px', fontSize: '11px', display: 'flex', alignItems: 'center' } },
                  option: { style: { padding: '2px 6px', fontSize: '10px' } }
                }
              }"
            />
          </div>

          <!-- 新建任务 -->
          <div v-if="canManage" class="create-action">
            <Button
              severity="primary"
              icon="pi pi-plus"
              @click="handleCreate"
              label="新建任务"
              size="small"
              class="create-btn"
              :pt="{
                root: { class: 'create-button-root' },
                label: { class: 'create-button-label' }
              }"
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
.jobs { }


.card-header {
  display: flex;
  justify-content: flex-end;
}

.group-select {
  width: 100px;
}

.create-action {
  display: flex;
  justify-content: center;
  margin-top: 20px;
  padding: 16px 0;
  border-top: 1px dashed var(--color-border-light);
}

.create-btn {
  background: var(--p-primary-50) !important;
  border: 1px solid var(--p-primary-100) !important;
  color: var(--p-primary-600) !important;
  padding: 8px 20px !important;
  font-weight: 600 !important;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1) !important;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.02) !important;
}

.create-btn:hover {
  background: var(--p-primary-100) !important;
  border-color: var(--p-primary-200) !important;
  transform: translateY(-1px);
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.05) !important;
}

.create-btn:active {
  transform: translateY(0);
}

.group-section {
  margin-bottom: 24px;
}

.group-section:last-of-type {
  margin-bottom: 0;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--color-border-light);
}

.group-icon {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: var(--color-border-light);
  color: var(--color-text-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  flex-shrink: 0;
}

.group-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.cron-label {
  font-size: 13px;
  color: var(--color-text-muted);
  cursor: default;
}

.time-text {
  font-size: 12px;
  color: var(--color-text-muted);
}

/* 执行节点样式 */
.target-any {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: var(--color-text-muted);
  font-style: italic;
}

.target-any i {
  font-size: 12px;
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
  max-width: 150px;
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

.target-node-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.target-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #0ea5e9;
  background: #f0f9ff;
  padding: 4px 10px 4px 8px;
  border-radius: 4px;
  border: 1px solid #bae6fd;
}

.target-tag i {
  font-size: 12px;
  color: #0284c7;
}

.action-row {
  display: flex;
  align-items: center;
  gap: 6px;
  justify-content: center;
}

.action-btn {
  width: 28px !important;
  height: 28px !important;
  padding: 0 !important;
  border: 1px solid var(--p-surface-200) !important;
  border-radius: 6px !important;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1) !important;
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
}

.action-btn:hover {
  transform: translateY(-1px);
  border-color: var(--p-surface-400) !important;
}

.btn-trigger {
  background: #f0f9ff !important; /* 直接使用颜色值确保生效 (info-50) */
  color: #0284c7 !important;
  border-color: #bae6fd !important;
}

.btn-trigger:hover {
  background: #e0f2fe !important;
  border-color: #7dd3fc !important;
  box-shadow: 0 2px 4px rgba(14, 165, 233, 0.1) !important;
}

.btn-delete {
  background: #fef2f2 !important; /* 直接使用颜色值确保生效 (danger-50) */
  color: #dc2626 !important;
  border-color: #fecaca !important;
}

.btn-delete:hover {
  background: #fee2e2 !important;
  border-color: #fca5a5 !important;
  box-shadow: 0 2px 4px rgba(239, 68, 68, 0.1) !important;
}

.action-row :deep(.p-button) {
  padding: 6px;
  margin: 0;
  transition: all 0.2s ease;
}

.action-row :deep(.p-button-outlined) {
  border-color: #e2e8f0;
  color: #64748b;
}

.action-row :deep(.p-button-outlined:hover) {
  background: #f8fafc;
  border-color: #cbd5e1;
  color: #334155;
}

.action-row :deep(.p-button-danger-outlined) {
  border-color: #fecaca;
  color: #dc2626;
}

.action-row :deep(.p-button-danger-outlined:hover) {
  background: #fef2f2;
  border-color: #f87171;
  color: #b91c1c;
}

.action-row :deep(.p-button-info-outlined) {
  border-color: #bfdbfe;
  color: #2563eb;
}

.action-row :deep(.p-button-info-outlined:hover) {
  background: #eff6ff;
  border-color: #60a5fa;
  color: #1d4ed8;
}

.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
  font-size: 11px;
  color: var(--color-text-muted);
}

.pagination :deep(.p-paginator-current) {
  display: flex;
  align-items: center;
}

.pagination :deep(.p-paginator) {
  padding: 0;
  gap: 2px;
}

.pagination :deep(.p-paginator-page) {
  min-width: 24px;
  height: 24px;
}

.pagination :deep(.p-paginator-first),
.pagination :deep(.p-paginator-prev),
.pagination :deep(.p-paginator-next),
.pagination :deep(.p-paginator-last) {
  min-width: 24px;
  height: 24px;
}

.pagination :deep(.p-select) {
  height: 24px;
}

.pagination :deep(.p-select-label) {
  font-size: 11px;
  padding: 0 8px;
  display: flex;
  align-items: center;
}

.group-select :deep(.p-select-label) {
  padding: 4px 8px;
}
</style>
