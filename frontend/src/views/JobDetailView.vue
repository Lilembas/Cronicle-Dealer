<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { jobsApi, eventsApi, type Job, type Event, type TriggerResponse } from '@/api'
import { ArrowLeft, Edit, VideoPlay, RefreshRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()
const job = ref<Job | null>(null)
const events = ref<Event[]>([])
const loading = ref(false)

// 触发结果 Dialog
const triggerVisible = ref(false)
const triggerResult = ref<TriggerResponse | null>(null)
const triggering = ref(false)

const getStatusType = (status: string) => {
  const map: Record<string, any> = {
    success: 'success',
    failed: 'danger',
    running: 'warning',
    queued: 'info',
    pending: 'info',
    aborted: 'info',
  }
  return map[status] || 'info'
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

const loadData = async () => {
  const id = route.params.id as string
  if (!id) return

  try {
    loading.value = true
    const [jobData, eventsData] = await Promise.all([
      jobsApi.get(id),
      eventsApi.list({ job_id: id, page: 1, page_size: 20 }),
    ])

    job.value = jobData as unknown as Job
    events.value = (eventsData as unknown as { data: Event[] }).data || []
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '加载任务详情失败')
  } finally {
    loading.value = false
  }
}

const handleTrigger = async () => {
  if (!job.value || triggering.value) return
  try {
    triggering.value = true
    const result = await jobsApi.trigger(job.value.id)
    triggerResult.value = result as unknown as TriggerResponse
    triggerVisible.value = true
    // 刷新执行记录列表
    loadData()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '任务触发失败')
  } finally {
    triggering.value = false
  }
}

const goToEvent = (eventId: string) => {
  triggerVisible.value = false
  router.push(`/events?highlight=${eventId}`)
}

const handleEdit = () => {
  if (job.value) {
    router.push(`/jobs/${job.value.id}`)
  }
}

const goBack = () => router.push('/jobs')

onMounted(() => {
  loadData()
})
</script>

<template>
  <div class="job-detail" v-loading="loading">
    <div class="page-header">
      <div class="left-actions">
        <el-button :icon="ArrowLeft" @click="goBack">返回</el-button>
        <h2 class="page-title">任务详情</h2>
      </div>
      <div class="right-actions">
        <el-button :icon="RefreshRight" @click="loadData">刷新</el-button>
        <el-button
          type="primary"
          :icon="VideoPlay"
          :loading="triggering"
          @click="handleTrigger"
        >立即触发</el-button>
        <el-button :icon="Edit" @click="handleEdit">编辑</el-button>
      </div>
    </div>

    <el-card v-if="job" class="job-card" shadow="never">
      <template #header>
        <div class="card-header">
          <span>{{ job.name }}</span>
          <el-tag :type="job.enabled ? 'success' : 'info'">{{ job.enabled ? '已启用' : '已禁用' }}</el-tag>
        </div>
      </template>

      <el-descriptions :column="2" border>
        <el-descriptions-item label="任务 ID">{{ job.id }}</el-descriptions-item>
        <el-descriptions-item label="分类">{{ job.category || '-' }}</el-descriptions-item>
        <el-descriptions-item label="Cron">{{ job.cron_expr }}</el-descriptions-item>
        <el-descriptions-item label="任务类型">{{ job.task_type }}</el-descriptions-item>
        <el-descriptions-item label="超时（秒）">{{ job.timeout }}</el-descriptions-item>
        <el-descriptions-item label="下次执行">
          {{ job.next_run_time ? new Date(job.next_run_time).toLocaleString('zh-CN') : '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="执行命令" :span="2">
          <pre class="command">{{ job.command }}</pre>
        </el-descriptions-item>
        <el-descriptions-item label="描述" :span="2">{{ job.description || '-' }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card class="events-card" shadow="never">
      <template #header>
        <div class="card-header">
          <span>最近执行记录</span>
          <el-tag size="small">{{ events.length }} 条</el-tag>
        </div>
      </template>

      <el-table :data="events" stripe>
        <el-table-column prop="id" label="Event ID" min-width="180" show-overflow-tooltip />
        <el-table-column label="状态" width="110" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="getStatusType(row.status)">
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="开始时间" width="180">
          <template #default="{ row }">
            {{ row.start_time ? new Date(row.start_time).toLocaleString('zh-CN') : '-' }}
          </template>
        </el-table-column>
        <el-table-column label="持续时长(秒)" width="120" align="right">
          <template #default="{ row }">
            {{ row.duration || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="退出码" width="90" align="center">
          <template #default="{ row }">
            {{ row.exit_code ?? '-' }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 触发结果 Dialog -->
    <el-dialog
      v-model="triggerVisible"
      title="任务已触发"
      width="480px"
      :close-on-click-modal="true"
    >
      <div v-if="triggerResult" class="trigger-result">
        <el-result icon="success" title="任务已成功加入队列">
          <template #sub-title>
            <p class="result-sub">任务正在等待 Worker 节点消费执行</p>
          </template>
        </el-result>
        <el-descriptions :column="1" border class="result-desc">
          <el-descriptions-item label="任务名称">{{ triggerResult.job_name }}</el-descriptions-item>
          <el-descriptions-item label="执行记录 ID">
            <span class="mono">{{ triggerResult.event_id }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="当前状态">
            <el-tag :type="getStatusType(triggerResult.status)" size="small">
              {{ getStatusText(triggerResult.status) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="入队时间">
            {{ new Date(triggerResult.queued_at * 1000).toLocaleString('zh-CN') }}
          </el-descriptions-item>
        </el-descriptions>
      </div>
      <template #footer>
        <el-button @click="triggerVisible = false">关闭</el-button>
        <el-button
          type="primary"
          @click="triggerResult && goToEvent(triggerResult.event_id)"
        >查看执行记录</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.job-detail {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  gap: 12px;
}

.left-actions,
.right-actions,
.card-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-title {
  margin: 0;
  font-size: 24px;
}

.job-card,
.events-card {
  border-radius: 12px;
  margin-bottom: 16px;
}

.command {
  margin: 0;
  white-space: pre-wrap;
}

.trigger-result {
  padding: 0 8px;
}

.result-sub {
  color: #666;
  margin: 4px 0 16px;
}

.result-desc {
  margin-top: 8px;
}

.mono {
  font-family: 'Courier New', monospace;
  font-size: 13px;
  word-break: break-all;
}
</style>
