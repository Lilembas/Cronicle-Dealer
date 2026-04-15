<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { jobsApi } from '@/api'
import { Plus, Edit, Delete, VideoPlay, RefreshRight, View, Clock } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

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
const jobsData = jobsDataRaw as unknown as { total: number; page: number; data: any[] } | undefined

// 当前选中分组
const selectedGroup = ref('')

// 计算所有分组
const allGroups = computed(() => {
  const jobs = jobsData.value?.data || []
  const groups = new Set<string>()
  jobs.forEach((job: any) => {
    groups.add(job.category || '未分组')
  })
  return Array.from(groups).sort()
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
  const map = new Map<string, any[]>()
  const jobs = groupedJobs.value.get(selectedGroup.value)
  if (jobs) map.set(selectedGroup.value, jobs)
  return map
})

// 分组颜色映射
const groupColorMap: Record<string, string> = {}
const groupColors = ['', 'success', 'warning', 'danger', 'info']
let colorIndex = 0

const getGroupColor = (group: string) => {
  if (!groupColorMap[group]) {
    groupColorMap[group] = groupColors[colorIndex % groupColors.length]
    colorIndex++
  }
  return groupColorMap[group]
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
          <el-table-column prop="cron_expr" label="Cron 表达式" width="140" />
          <el-table-column label="状态" width="90" align="center">
            <template #default="{ row }">
              <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
                {{ row.enabled ? '启用' : '禁用' }}
              </el-tag>
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
