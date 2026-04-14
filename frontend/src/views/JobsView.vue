<script setup lang="ts">
import { ref } from 'vue'
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
    const result = await jobsApi.trigger(id) as unknown as import('@/api').TriggerResponse
    ElMessage({
      message: `任务 "${name}" 已入队，Event ID: ${result.event_id}`,
      type: 'success',
      duration: 5000,
      showClose: true,
    })
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '触发失败')
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
    </div>

    <el-card>
      <el-table :data="jobsData?.data || []" v-loading="isLoading" stripe>
        <el-table-column prop="name" label="任务名称" min-width="200" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="cron_expr" label="Cron 表达式" width="150" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'">
              {{ row.enabled ? '已启用' : '已禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="执行统计" width="150">
          <template #default="{ row }">
            <div class="text-sm">
              <span class="text-green-600">✓ {{ row.success_runs }}</span> /
              <span class="text-red-600">✗ {{ row.failed_runs }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="next_run_time" label="下次执行" width="180">
          <template #default="{ row }">
            {{ row.next_run_time ? new Date(row.next_run_time).toLocaleString('zh-CN') : '-' }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="320" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button size="small" :icon="View" @click="handleDetail(row.id)">
                详情
              </el-button>
              <el-button size="small" :icon="Clock" @click="handleHistory(row.id)">
                历史
              </el-button>
              <el-button size="small" :icon="VideoPlay" @click="handleTrigger(row.id, row.name)">
                触发
              </el-button>
              <el-button size="small" :icon="Edit" @click="handleEdit(row.id)">
                编辑
              </el-button>
              <el-button size="small" type="danger" :icon="Delete" @click="handleDelete(row.id, row.name)">
                删除
              </el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>

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
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #333;
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.pagination {
  margin-top: 20px;
  display: flex;
  justify-content: flex-end;
}

.text-sm {
  font-size: 12px;
}

.text-green-600 {
  color: #67c23a;
}

.text-red-600 {
  color: #f56c6c;
}
</style>
