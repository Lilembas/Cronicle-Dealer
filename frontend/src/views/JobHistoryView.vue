<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { eventsApi, jobsApi, type Event } from '@/api'
import { RefreshRight, ArrowLeft } from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()

const jobId = route.params.id as string

// 获取任务信息
const { data: jobData } = useQuery({
  queryKey: ['job', jobId],
  queryFn: () => jobsApi.get(jobId),
})

// 分页参数
const pagination = ref({
  page: 1,
  pageSize: 20,
})

// 获取该任务的执行记录
const { data: eventsDataRaw, isLoading, refetch } = useQuery({
  queryKey: ['job-events', jobId, pagination],
  queryFn: () => eventsApi.list({
    page: pagination.value.page,
    page_size: pagination.value.pageSize,
    job_id: jobId,
  }),
})
const eventsData = eventsDataRaw as unknown as { total: number; data: Event[] } | undefined

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

// 返回上一页
const goBack = () => {
  router.push('/jobs')
}

// 查看日志
const viewLog = (event: Event) => {
  router.push(`/logs/${event.id}`)
}

// 分页变化
const handlePageChange = (page: number) => {
  pagination.value.page = page
}
</script>

<template>
  <div class="job-history">
    <div class="page-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" @click="goBack">返回</el-button>
        <span class="page-title">{{ jobData?.data?.name || '任务' }} - 执行历史</span>
      </div>
      <el-button :icon="RefreshRight" @click="refetch">刷新</el-button>
    </div>

    <!-- 执行记录列表 -->
    <el-card class="table-card" shadow="never">
      <el-table
        :data="eventsData?.data || []"
        v-loading="isLoading"
        stripe
        class="events-table"
        :row-key="(row: Event) => row.id"
      >
        <el-table-column prop="id" label="Event ID" min-width="180">
          <template #default="{ row }">
            <el-text type="primary" class="event-id" @click="viewLog(row)">{{ row.id }}</el-text>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="开始时间" width="180">
          <template #default="{ row }">
            {{ row.start_time ? new Date(row.start_time).toLocaleString('zh-CN') : '-' }}
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

        <el-table-column prop="node_name" label="执行节点" width="120" show-overflow-tooltip />

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
        v-if="!isLoading && (!eventsData?.data || (eventsData?.data?.length || 0) === 0)"
        description="暂无执行记录"
      />
    </el-card>
  </div>
</template>

<style scoped>
.job-history {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-title {
  font-size: 20px;
  font-weight: 600;
  color: #333;
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
</style>
