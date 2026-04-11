<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { eventsApi, type Event } from '@/api'
import { RefreshRight, View, Filter, CircleClose } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()

// 筛选条件
const filters = ref({
  status: '',
  jobId: '',
  startDate: '',
  endDate: '',
})

// 分页参数
const pagination = ref({
  page: 1,
  pageSize: 20,
})

// 获取执行记录列表
const { data: eventsData, isLoading, refetch } = useQuery({
  queryKey: ['events', pagination, filters],
  queryFn: () => eventsApi.list({
    page: pagination.value.page,
    page_size: pagination.value.pageSize,
    status: filters.value.status || undefined,
    job_id: filters.value.jobId || undefined,
  }),
})

// 状态选项
const statusOptions = [
  { label: '全部', value: '' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '运行中', value: 'running' },
  { label: '已排队', value: 'queued' },
]

// 状态标签类型
const getStatusType = (status: string) => {
  const map: Record<string, any> = {
    success: 'success',
    failed: 'danger',
    running: 'warning',
    queued: 'info',
  }
  return map[status] || 'info'
}

// 状态文本
const getStatusText = (status: string) => {
  const map: Record<string, string> = {
    success: '成功',
    failed: '失败',
    running: '运行中',
    queued: '已排队',
    aborted: '已中止',
  }
  return map[status] || status
}

// 格式化持续时间
const formatDuration = (seconds: number) => {
  if (!seconds) return '-'
  if (seconds < 60) return `${seconds.toFixed(2)}秒`
  if (seconds < 3600) return `${(seconds / 60).toFixed(2)}分钟`
  return `${(seconds / 3600).toFixed(2)}小时`
}

// 查看详情
const viewDetail = (event: Event) => {
  router.push(`/logs/${event.id}`)
}

const canAbort = (status: string) => status === 'running' || status === 'pending' || status === 'queued'

const handleAbort = async (event: Event) => {
  try {
    await ElMessageBox.confirm(`确认中止任务 ${event.id} 吗？`, '中止确认', {
      confirmButtonText: '确认',
      cancelButtonText: '取消',
      type: 'warning',
    })

    await eventsApi.abort(event.id)
    ElMessage.success('中止请求已提交')
    refetch()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('中止失败')
    }
  }
}

// 应用筛选
const applyFilter = () => {
  pagination.value.page = 1
  refetch()
}

// 重置筛选
const resetFilter = () => {
  filters.value = {
    status: '',
    jobId: '',
    startDate: '',
    endDate: '',
  }
  pagination.value.page = 1
  refetch()
}

// 分页变化
const handlePageChange = (page: number) => {
  pagination.value.page = page
}
</script>

<template>
  <div class="events">
    <div class="page-header">
      <h2 class="page-title">执行记录</h2>
      <el-button :icon="RefreshRight" @click="refetch">刷新</el-button>
    </div>

    <!-- 筛选栏 -->
    <el-card class="filter-card" shadow="never">
      <el-form :inline="true" label-width="80px">
        <el-form-item label="状态">
          <el-select v-model="filters.status" placeholder="选择状态" style="width: 150px">
            <el-option
              v-for="option in statusOptions"
              :key="option.value"
              :label="option.label"
              :value="option.value"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="任务ID">
          <el-input
            v-model="filters.jobId"
            placeholder="输入任务ID"
            clearable
            style="width: 200px"
          />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :icon="Filter" @click="applyFilter">筛选</el-button>
          <el-button @click="resetFilter">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 执行记录列表 -->
    <el-card class="table-card" shadow="never">
      <el-table
        :data="eventsData?.data || []"
        v-loading="isLoading"
        stripe
        class="events-table"
      >
        <el-table-column prop="id" label="Event ID" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <el-text type="primary" class="event-id">{{ row.id }}</el-text>
          </template>
        </el-table-column>

        <el-table-column prop="job_id" label="任务ID" min-width="150" show-overflow-tooltip />

        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="执行时间" width="180">
          <template #default="{ row }">
            <div v-if="row.start_time">
              <div>{{ new Date(row.start_time).toLocaleString('zh-CN') }}</div>
              <div v-if="row.end_time" class="text-sm text-gray">
                至 {{ new Date(row.end_time).toLocaleString('zh-CN') }}
              </div>
            </div>
            <span v-else>-</span>
          </template>
        </el-table-column>

        <el-table-column label="持续时间" width="120">
          <template #default="{ row }">
            {{ formatDuration(row.duration) }}
          </template>
        </el-table-column>

        <el-table-column label="退出码" width="100" align="center">
          <template #default="{ row }">
            <span v-if="row.exit_code !== undefined" :class="row.exit_code === 0 ? 'text-green' : 'text-red'">
              {{ row.exit_code }}
            </span>
            <span v-else>-</span>
          </template>
        </el-table-column>

        <el-table-column prop="cpu_percent" label="CPU" width="100" align="right">
          <template #default="{ row }">
            <span v-if="row.cpu_percent !== undefined">{{ row.cpu_percent.toFixed(1) }}%</span>
            <span v-else>-</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="210" fixed="right">
          <template #default="{ row }">
            <el-button
              type="primary"
              size="small"
              :icon="View"
              @click="viewDetail(row)"
            >
              查看日志
            </el-button>
            <el-button
              v-if="canAbort(row.status)"
              type="danger"
              size="small"
              :icon="CircleClose"
              @click="handleAbort(row)"
            >
              中止
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :total="eventsData?.total || 0"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="handlePageChange"
        />
      </div>

      <el-empty
        v-if="!isLoading && (!eventsData?.data || eventsData.data.length === 0)"
        description="暂无执行记录"
      />
    </el-card>
  </div>
</template>

<style scoped>
.events {
  padding: 24px;
  max-width: 1600px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
  margin: 0;
}

.filter-card {
  border-radius: 16px;
  border: 1px solid #e2e8f0;
  margin-bottom: 20px;
}

.table-card {
  border-radius: 16px;
  border: 1px solid #e2e8f0;
}

.events-table {
  width: 100%;
}

.event-id {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13px;
  cursor: pointer;
}

.event-id:hover {
  text-decoration: underline;
}

.text-sm {
  font-size: 12px;
}

.text-gray {
  color: #64748b;
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

/* 响应式设计 */
@media (max-width: 768px) {
  .events {
    padding: 16px;
  }

  .page-title {
    font-size: 24px;
  }

  .filter-card :deep(.el-form) {
    flex-direction: column;
  }

  .filter-card :deep(.el-form-item) {
    width: 100%;
    margin-right: 0;
  }

  .events-table :deep(.el-table__body-wrapper) {
    overflow-x: auto;
  }
}
</style>
