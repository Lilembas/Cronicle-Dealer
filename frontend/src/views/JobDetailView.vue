<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { jobsApi, eventsApi, type Job, type Event } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import ProgressSpinner from 'primevue/progressspinner'

const wsStore = useWebSocketStore()


const route = useRoute()
const router = useRouter()
const job = ref<Job | null>(null)
const events = ref<Event[]>([])
const loading = ref(false)
const triggering = ref(false)

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
    showToast({ severity: 'error', summary: '加载失败', detail: error.response?.data?.error || '加载任务详情失败', life: 5000 })
  } finally {
    loading.value = false
  }
}

const handleTrigger = async () => {
  if (!job.value || triggering.value) return

  showConfirm({
    message: `确定要触发任务 "${job.value.name}" 吗？`,
    header: '确认触发',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: 'info' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        triggering.value = true
        const result = await jobsApi.trigger(job.value!.id) as unknown as import('@/api').TriggerResponse
        showToast({ severity: 'success', summary: '任务已入队', detail: `Event ID: ${result.event_id}`, life: 5000 })
        loadData()
      } catch (error: any) {
        showToast({ severity: 'error', summary: '触发失败', detail: error.response?.data?.error || '任务触发失败', life: 5000 })
      } finally {
        triggering.value = false
      }
    },
  })
}

const handleEdit = () => {
  if (job.value) {
    router.push(`/jobs/${job.value.id}`)
  }
}

const goBack = () => router.push('/jobs')

const handleTaskStatus = (data: any) => {
  if (job.value && data.job_id === job.value.id) {
    loadData()
  }
}

onMounted(() => {
  loadData()
  wsStore.onMessage('task_status', handleTaskStatus)
})

onUnmounted(() => {
  wsStore.offMessage('task_status', handleTaskStatus)
})
</script>

<template>
  <div class="job-detail">
    <div v-if="loading" class="flex justify-center py-16">
      <ProgressSpinner style="width:50px;height:50px" strokeWidth="4" />
    </div>

    <template v-else>
      <div class="page-header">
        <div class="left-actions">
          <Button icon="pi pi-arrow-left" text @click="goBack" label="返回" />
          <h2 class="page-title">任务详情</h2>
        </div>
        <div class="right-actions">
          <Button icon="pi pi-refresh" text @click="loadData" label="刷新" />
          <Button severity="info" icon="pi pi-play" :loading="triggering" @click="handleTrigger" label="立即触发" />
          <Button icon="pi pi-pencil" text @click="handleEdit" label="编辑" />
        </div>
      </div>

      <Card v-if="job" class="job-card">
        <template #title>
          <div class="card-header">
            <span>{{ job.name }}</span>
            <Tag :value="job.enabled ? '已启用' : '已禁用'" :severity="job.enabled ? 'success' : 'secondary'" />
          </div>
        </template>
        <template #content>
          <div class="desc-grid">
            <div class="desc-label">任务 ID</div>
            <div class="desc-value">{{ job.id }}</div>
            <div class="desc-label">分类</div>
            <div class="desc-value">{{ job.category || '-' }}</div>
            <div class="desc-label">Cron</div>
            <div class="desc-value">{{ job.cron_expr }}</div>
            <div class="desc-label">任务类型</div>
            <div class="desc-value">{{ job.task_type }}</div>
            <div class="desc-label">超时（秒）</div>
            <div class="desc-value">{{ job.timeout }}</div>
            <div class="desc-label">下次执行</div>
            <div class="desc-value">{{ job.next_run_time ? new Date(job.next_run_time).toLocaleString('zh-CN') : '-' }}</div>
            <div class="desc-label">描述</div>
            <div class="desc-value col-span-2">{{ job.description || '-' }}</div>
            <div class="desc-label">执行命令</div>
            <div class="desc-value col-span-2"><pre class="command">{{ job.command }}</pre></div>
          </div>
        </template>
      </Card>

      <Card class="events-card">
        <template #title>
          <div class="card-header">
            <span>最近执行记录</span>
            <Tag :value="`${events.length} 条`" severity="info" />
          </div>
        </template>
        <template #content>
          <DataTable :value="events" stripedRows>
            <Column field="id" header="Event ID" style="min-width: 180px">
              <template #body="{ data }">
                <span class="event-link" @click="router.push(`/logs/${data.id}`)">{{ data.id.split('_').slice(-1)[0] }}</span>
              </template>
            </Column>
            <Column header="状态" style="width: 110px" alignHeader="center" align="center">
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
                {{ data.start_time ? new Date(data.start_time).toLocaleString('zh-CN') : '-' }}
              </template>
            </Column>
            <Column header="持续时长(秒)" style="width: 120px" alignHeader="right" align="right">
              <template #body="{ data }">
                {{ data.duration || '-' }}
              </template>
            </Column>
            <Column header="退出码" style="width: 90px" alignHeader="center" align="center">
              <template #body="{ data }">
                {{ data.exit_code ?? '-' }}
              </template>
            </Column>
          </DataTable>
        </template>
      </Card>
    </template>
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

.desc-grid {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0;
}

.desc-label {
  padding: 10px 16px;
  background: #f8fafc;
  font-weight: 500;
  font-size: 14px;
  color: #64748b;
  border-bottom: 1px solid #e2e8f0;
  border-right: 1px solid #e2e8f0;
}

.desc-value {
  padding: 10px 16px;
  font-size: 14px;
  border-bottom: 1px solid #e2e8f0;
}

.col-span-2 {
  grid-column: span 2;
}

.col-span-2.desc-label {
  border-right: none;
}

.event-link {
  color: #409eff;
  cursor: pointer;
}

.event-link:hover {
  text-decoration: underline;
}

.command {
  font-family: 'Courier New', monospace;
  font-size: 13px;
  word-break: break-all;
  margin: 0;
}
</style>
