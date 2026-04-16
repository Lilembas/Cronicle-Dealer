<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { jobsApi, nodesApi, type Node } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { Plus, Edit, Delete, VideoPlay, RefreshRight, View, Clock, CircleCheckFilled, CircleCloseFilled, Loading } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const wsStore = useWebSocketStore()
const queryClient = useQueryClient()

const router = useRouter()

// 分页参数
const pagination = ref({
  page: 1,
  pageSize: 20,
})

// 获取任务列表
const { data: jobsDataRaw, isLoading, refetch } = useQuery({
  queryKey: ['jobs', pagination],
  queryFn: () => jobsApi.list({
    page: pagination.value.page,
    page_size: pagination.value.pageSize,
  }),
})
const jobsData = jobsDataRaw as unknown as import('vue').Ref<{ total: number; page: number; data: any[] } | undefined>

// 当前选中分组
const selectedGroup = ref('')

// 计算所有分组
const allGroups = computed(() => {
  const jobs = jobsData.value?.data || []
  return Array.from(new Set(jobs.map((job: any) => job.category || '未分组'))).sort()
})

// 按分组归类任务
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

// 过滤后的分组
const filteredGroups = computed(() => {
  if (!selectedGroup.value) return groupedJobs.value
  const jobs = groupedJobs.value.get(selectedGroup.value)
  return jobs ? new Map([[selectedGroup.value, jobs]]) : new Map()
})

// 分组颜色映射
const groupColorMap: Record<string, string> = {}
const groupColors = ['success', 'warning', 'danger', 'info']
let colorIndex = 0

const getGroupColor = (group: string) => {
  if (!groupColorMap[group]) {
    groupColorMap[group] = groupColors[colorIndex % groupColors.length]
    colorIndex++
  }
  return groupColorMap[group]
}

// 节点ID -> hostname 映射
const nodesMap = ref<Map<string, string>>(new Map())

const loadNodes = async () => {
  try {
    const all = await nodesApi.list({}) as unknown as Node[]
    nodesMap.value = new Map((all || []).map(n => [n.id, n.hostname]))
  } catch {
    // 加载失败不影响主要功能
  }
}

// 执行目标格式化
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

// Cron表达式转人类可读描述
const weekDays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

const formatCron = (expr: string): string => {
  if (!expr) return expr
  const parts = expr.trim().split(/\s+/)
  if (parts.length !== 5) return expr
  const [min, hour, dom, month, dow] = parts

  if (expr === '* * * * *') return '每分钟'

  // */N * * * * → 每N分钟
  if (/^\*\/\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    return `每${min.split('/')[1]}分钟`
  }
  // N/M * * * * → 从第N分开始每M分钟
  if (/^\d+\/\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    const [start, step] = min.split('/')
    return `从第${start}分起每${step}分钟`
  }
  // 0 */N * * * → 每N小时
  if (min === '0' && /^\*\/\d+$/.test(hour) && dom === '*' && month === '*' && dow === '*') {
    return `每${hour.split('/')[1]}小时`
  }
  // M * * * * → 每小时第M分
  if (/^\d+$/.test(min) && hour === '*' && dom === '*' && month === '*' && dow === '*') {
    return `每小时第${min}分`
  }
  // M H * * D → 每周X H:M
  if (/^\d+$/.test(min) && /^\d+$/.test(hour) && dom === '*' && month === '*' && /^\d+$/.test(dow)) {
    const label = weekDays[parseInt(dow) % 7]
    return `每${label} ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  // M H D * * → 每月D日 H:M
  if (/^\d+$/.test(min) && /^\d+$/.test(hour) && /^\d+$/.test(dom) && month === '*' && dow === '*') {
    return `每月${dom}日 ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  // M H * * * → 每天 H:M
  if (/^\d+$/.test(min) && /^\d+$/.test(hour) && dom === '*' && month === '*' && dow === '*') {
    return `每天 ${hour.padStart(2, '0')}:${min.padStart(2, '0')}`
  }
  return expr
}

// 新建任务
const handleCreate = () => {
  router.push('/jobs/new')
}

// 编辑任务
const handleEdit = (id: string) => {
  router.push(`/jobs/${id}`)
}

const handleDetail = (id: string) => {
  router.push(`/jobs/${id}/detail`)
}

const handleHistory = (id: string) => {
  router.push(`/jobs/${id}/history`)
}

// 删除任务
const handleDelete = async (id: string, name: string) => {
  try {
    await ElMessageBox.confirm(`确定要删除任务 "${name}" 吗？`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })

    await jobsApi.delete(id)
    ElMessage.success('删除成功')
    refetch()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 触发任务
const handleTrigger = async (id: string, name: string) => {
  try {
    await ElMessageBox.confirm(`确定要触发任务 "${name}" 吗？`, '确认触发', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info',
    })

    const result = await jobsApi.trigger(id) as unknown as import('@/api').TriggerResponse
    ElMessage({
      message: `任务 "${name}" 已入队，Event ID: ${result.event_id}`,
      type: 'success',
      duration: 5000,
      showClose: true,
    })
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '触发失败')
    }
  }
}

// 分页变化
const handlePageChange = (page: number) => {
  pagination.value.page = page
}

// WebSocket 任务状态更新处理
const handleTaskStatus = async () => {
  // 标记查询为失效状态，Vue Query 会立即重新获取数据
  queryClient.invalidateQueries({ queryKey: ['jobs'] })
  // 强制 Vue 立即刷新 DOM
  await nextTick()
}

// 组件挂载 - 设置 WebSocket 监听
onMounted(() => {
  wsStore.onMessage('task_status', handleTaskStatus)
  loadNodes()
})

// 组件卸载 - 移除 WebSocket 监听
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
        <el-button :icon="RefreshRight" @click="refetch">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="handleCreate">新建任务</el-button>
      </div>
      <el-select v-model="selectedGroup" placeholder="全部分组" clearable style="width: 160px">
        <el-option label="全部分组" value="" />
        <el-option
          v-for="group in allGroups"
          :key="group"
          :label="group"
          :value="group"
        />
      </el-select>
    </div>

    <el-card v-loading="isLoading">
      <!-- 按分组渲染 -->
      <div v-for="[group, jobs] in filteredGroups" :key="group" class="group-section">
        <div class="group-header">
          <el-tag :type="getGroupColor(group)" size="small" effect="plain">{{ group }}</el-tag>
          <span class="group-count">{{ jobs.length }} 个任务</span>
        </div>
        <el-table :data="jobs" stripe size="small" style="width: 100%" class="group-table">
          <el-table-column prop="name" label="任务名称" min-width="120" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="job-name-link" @click="handleDetail(row.id)">{{ row.name }}</span>
            </template>
          </el-table-column>
          <el-table-column label="执行节点" width="150">
            <template #default="{ row }">
              <template v-if="formatTarget(row).type === 'any'">
                <el-tag size="small" type="info" effect="plain">任意节点</el-tag>
              </template>
              <template v-else-if="formatTarget(row).type === 'node'">
                <el-tag size="small" type="primary" effect="plain" style="max-width: 130px; overflow: hidden; text-overflow: ellipsis">
                  {{ formatTarget(row).label }}
                </el-tag>
              </template>
              <template v-else-if="formatTarget(row).type === 'tags'">
                <div class="target-tags">
                  <el-tag
                    v-for="tag in formatTarget(row).tags"
                    :key="tag"
                    size="small"
                    type="warning"
                    effect="plain"
                  >{{ tag }}</el-tag>
                </div>
              </template>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="90" align="center">
            <template #default="{ row }">
              <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
                {{ row.enabled ? '启用' : '禁用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="最后执行" width="100" align="center">
            <template #default="{ row }">
              <span v-if="row.last_status && row.last_status !== '-'" :class="['status-badge', `status-${row.last_status}`]">
                <el-icon v-if="row.last_status === 'success'"><CircleCheckFilled /></el-icon>
                <el-icon v-else-if="row.last_status === 'failed'"><CircleCloseFilled /></el-icon>
                <el-icon v-else-if="row.last_status === 'running'" class="is-loading"><Loading /></el-icon>
                <el-icon v-else><Clock /></el-icon>
                <span>{{ getStatusText(row.last_status) }}</span>
              </span>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column prop="cron_expr" label="执行计划" width="160">
            <template #default="{ row }">
              <el-tooltip :content="row.cron_expr" placement="top">
                <span class="cron-label">{{ formatCron(row.cron_expr) }}</span>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column prop="next_run_time" label="下次执行" width="170">
            <template #default="{ row }">
              {{ row.next_run_time ? new Date(row.next_run_time).toLocaleString('zh-CN') : '-' }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <div class="action-row">
                <el-tooltip content="详情" placement="top">
                  <el-button size="small" :icon="View" @click="handleDetail(row.id)" />
                </el-tooltip>
                <el-tooltip content="历史" placement="top">
                  <el-button size="small" :icon="Clock" @click="handleHistory(row.id)" />
                </el-tooltip>
                <el-tooltip content="触发" placement="top">
                  <el-button size="small" type="primary" :icon="VideoPlay" @click="handleTrigger(row.id, row.name)" />
                </el-tooltip>
                <el-tooltip content="编辑" placement="top">
                  <el-button size="small" :icon="Edit" @click="handleEdit(row.id)" />
                </el-tooltip>
                <el-tooltip content="删除" placement="top">
                  <el-button size="small" type="danger" :icon="Delete" @click="handleDelete(row.id, row.name)" />
                </el-tooltip>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 分页 -->
      <div class="pagination">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="jobsData?.total || 0"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="handlePageChange"
        />
      </div>

      <el-empty v-if="!isLoading && (!jobsData?.data || jobsData.data.length === 0)" description="暂无任务数据" />
    </el-card>
  </div>
</template>

<style scoped>
.jobs {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.header-actions {
  display: flex;
  gap: 10px;
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
  color: #909399;
}

.group-table :deep(.el-table__header-wrapper th) {
  background-color: #fafafa;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.job-name-link {
  color: #409eff;
  cursor: pointer;
}

.job-name-link:hover {
  text-decoration: underline;
}

.cron-label {
  font-size: 13px;
  color: #94a3b8;
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
  gap: 4px;
  flex-wrap: nowrap;
}

.action-row :deep(.el-button) {
  padding: 4px;
  margin: 0;
}

</style>
