<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject, type Ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { statsApi, nodesApi, eventsApi, jobsApi } from '@/api'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import ProgressBar from 'primevue/progressbar'
import Card from 'primevue/card'
import Button from 'primevue/button'
import { useWebSocketStore } from '@/stores/websocket'
import { useSystemStore } from '@/stores/system'

const stats = ref<any>(null)
const nodes = ref<any[]>([])
const runningEvents = ref<any[]>([])
const upcomingJobs = ref<any[]>([])
const statsLoading = ref(true)
const nodesLoading = ref(true)
const wsStore = useWebSocketStore()
const systemStore = useSystemStore()
const router = useRouter()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')

const isMasterNode = (node: any) => {
  if (!node || !node.tags) return false
  return node.tags === 'master' || node.tags.includes('master')
}

const loadData = async () => {
  try {
    statsLoading.value = true
    nodesLoading.value = true

    const [statsData, nodesData, runningData, jobsData] = await Promise.all([
      statsApi.get(),
      nodesApi.list(),
      eventsApi.list({ status: 'running', page_size: 5 }),
      jobsApi.list({ page_size: 100, enabled: true })
    ])

    stats.value = statsData
    nodes.value = (nodesData as any) || []
    runningEvents.value = (runningData as any).data || []
    
    // Process upcoming jobs
    upcomingJobs.value = ((jobsData as any).data || [])
      .filter((j: any) => j.enabled && j.next_run_time && new Date(j.next_run_time).getTime() > systemStore.currentTime)
      .sort((a: any, b: any) => new Date(a.next_run_time).getTime() - new Date(b.next_run_time).getTime())
      .slice(0, 5)
  } catch (error) {
    console.error('加载数据失败:', error)
  } finally {
    statsLoading.value = false
    nodesLoading.value = false
  }
}

const refetchStats = async () => {
  await loadData()
}

const updateNode = (nodeData: any) => {
  const index = nodes.value.findIndex(n => n.id === nodeData.node_id)
  if (index !== -1) {
    nodes.value[index] = {
      ...nodes.value[index],
      status: nodeData.status,
      cpu_usage: nodeData.cpu_usage,
      memory_percent: nodeData.memory_percent,
      running_jobs: nodeData.running_jobs
    }
  } else {
    loadData()
  }
}

const handleTaskStatus = () => {
  refetchStats()
}

const handleAbort = async (event: any) => {
  try {
    await eventsApi.abort(event.id)
    handleTaskStatus()
  } catch (error) {
    console.error('中止任务失败:', error)
  }
}

const handleNodeStatus = (data: any) => {
  updateNode(data)
  statsApi.get().then(statsData => {
    stats.value = statsData
  })
}

const getProgressColor = (percentage: number) => {
  if (percentage < 60) return '#10b981'
  if (percentage < 80) return '#f59e0b'
  return '#ef4444'
}

const sortedNodes = computed(() => {
  if (!nodes.value) return []
  return [...nodes.value].sort((a: any, b: any) => {
    const aIsMaster = isMasterNode(a)
    const bIsMaster = isMasterNode(b)
    if (aIsMaster && !bIsMaster) return -1
    if (!aIsMaster && bIsMaster) return 1
    if (a.status === 'online' && b.status !== 'online') return -1
    if (a.status !== 'online' && b.status === 'online') return 1
    return new Date(a.registered_at).getTime() - new Date(b.registered_at).getTime()
  })
})

onMounted(async () => {
  await loadData()
  wsStore.onMessage('task_status', handleTaskStatus)
  wsStore.onMessage('node_status', handleNodeStatus)
  if (globalRefreshHandler) {
    globalRefreshHandler.value = refetchStats
  }
})

onUnmounted(() => {
  wsStore.offMessage('task_status', handleTaskStatus)
  wsStore.offMessage('node_status', handleNodeStatus)
  if (globalRefreshHandler) {
    globalRefreshHandler.value = null
  }
})
</script>

<template>
  <div class="dashboard">
    <!-- Row 1: Quick Stats -->
    <div class="stats-grid mb-6">
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-blue-50 text-blue-500">
              <i class="pi pi-calendar-plus text-xl"></i>
            </div>
            <div class="flex-1">
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">总任务数</div>
              <div class="flex items-baseline gap-2">
                <span class="text-2xl font-bold">{{ stats?.total_jobs || 0 }}</span>
                <span class="text-xs text-green-500 font-medium">Active: {{ stats?.enabled_jobs || 0 }}</span>
              </div>
            </div>
          </div>
        </template>
      </Card>
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-green-50 text-green-500">
              <i class="pi pi-check-circle text-xl"></i>
            </div>
            <div class="flex-1">
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">成功率 (24h)</div>
              <div class="flex items-baseline gap-2">
                <span class="text-2xl font-bold text-green-600">{{ stats?.total_events ? Math.round((stats.success_events / stats.total_events) * 100) : 100 }}%</span>
                <span class="text-xs text-gray-400">Total: {{ stats?.total_events || 0 }}</span>
              </div>
            </div>
          </div>
        </template>
      </Card>
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-amber-50 text-amber-500">
              <i class="pi pi-spin pi-spinner text-xl"></i>
            </div>
            <div class="flex-1">
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">当前运行</div>
              <div class="flex items-baseline gap-2">
                <span class="text-2xl font-bold text-amber-600">{{ stats?.running_events || 0 }}</span>
                <span class="text-xs text-gray-400">Loading...</span>
              </div>
            </div>
          </div>
        </template>
      </Card>
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-purple-50 text-purple-500">
              <i class="pi pi-desktop text-xl"></i>
            </div>
            <div class="flex-1">
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">在线节点</div>
              <div class="flex items-baseline gap-2">
                <span class="text-2xl font-bold text-purple-600">{{ stats?.online_nodes || 0 }}</span>
                <span class="text-xs text-red-500 font-medium">Offline: {{ stats?.offline_nodes || 0 }}</span>
              </div>
            </div>
          </div>
        </template>
      </Card>
    </div>

    <!-- Row 2: Node Health & Cluster Distribution -->
    <div class="grid grid-cols-12 gap-6 mb-6">
      <div class="col-span-12 lg:col-span-8">
        <Card class="premium-card h-full">
          <template #title>
            <div class="flex justify-between items-center px-2">
              <div class="flex items-center gap-2">
                <div class="w-1.5 h-6 bg-blue-500 rounded-full"></div>
                <h3 class="text-sm font-bold uppercase tracking-tight">节点运行状态</h3>
              </div>
              <Tag :value="`${nodes?.length || 0} Nodes Connected`" severity="info" class="text-[10px]" />
            </div>
          </template>
          <template #content>
            <DataTable :value="sortedNodes" stripedRows class="nodes-table-mini" :loading="nodesLoading" emptyMessage="暂无节点数据">
              <Column field="hostname" header="HOSTNAME" style="min-width: 140px">
                <template #body="{ data }">
                  <div class="flex items-center gap-2">
                    <div class="status-indicator">
                      <span :class="['status-dot', data.status === 'online' ? 'status-online' : 'status-offline']"></span>
                      <span v-if="data.status === 'online'" class="status-pulse"></span>
                    </div>
                    <i :class="[isMasterNode(data) ? 'pi pi-shield text-amber-500' : 'pi pi-desktop text-blue-400']" class="text-[10px]"></i>
                    <span class="hostname-text">{{ data.hostname }}</span>
                  </div>
                </template>
              </Column>
              <Column header="CPU" style="width: 130px">
                <template #body="{ data }">
                  <div class="usage-metric" v-if="data.status === 'online'">
                    <ProgressBar :value="Math.round(data.cpu_usage || 0)" :showValue="false" class="mini-progress-bar" :class="getProgressColor(data.cpu_usage)" />
                    <span class="usage-label-text">{{ Math.round(data.cpu_usage || 0) }}%</span>
                  </div>
                  <span v-else class="text-gray-300">-</span>
                </template>
              </Column>
              <Column header="MEMORY" style="width: 130px">
                <template #body="{ data }">
                  <div class="usage-metric" v-if="data.status === 'online'">
                    <ProgressBar :value="Math.round(data.memory_percent || 0)" :showValue="false" class="mini-progress-bar" :class="getProgressColor(data.memory_percent)" />
                    <span class="usage-label-text">{{ Math.round(data.memory_percent || 0) }}%</span>
                  </div>
                  <span v-else class="text-gray-300">-</span>
                </template>
              </Column>
            </DataTable>
          </template>
        </Card>
      </div>

      <div class="col-span-12 lg:col-span-4">
        <Card class="premium-card h-full">
          <template #title>
            <div class="flex items-center gap-2 px-2">
              <div class="w-1.5 h-6 bg-purple-500 rounded-full"></div>
              <h3 class="text-sm font-bold uppercase tracking-tight">集群资源分布</h3>
            </div>
          </template>
          <template #content>
            <div class="flex flex-col gap-6 py-2 px-2">
              <div class="resource-block">
                <div class="flex justify-between items-center mb-2">
                  <span class="text-xs font-semibold text-gray-500">总体 CPU 利用率</span>
                  <span class="text-xs font-bold">{{ nodes.length > 0 ? Math.round(nodes.reduce((acc, n) => acc + (n.cpu_usage || 0), 0) / nodes.length) : 0 }}%</span>
                </div>
                <ProgressBar :value="nodes.length > 0 ? nodes.reduce((acc, n) => acc + (n.cpu_usage || 0), 0) / nodes.length : 0" :showValue="false" class="h-2 rounded-full" />
              </div>
              <div class="resource-block">
                <div class="flex justify-between items-center mb-2">
                  <span class="text-xs font-semibold text-gray-500">总体 内存 利用率</span>
                  <span class="text-xs font-bold">{{ nodes.length > 0 ? Math.round(nodes.reduce((acc, n) => acc + (n.memory_percent || 0), 0) / nodes.length) : 0 }}%</span>
                </div>
                <ProgressBar :value="nodes.length > 0 ? nodes.reduce((acc, n) => acc + (n.memory_percent || 0), 0) / nodes.length : 0" :showValue="false" class="h-2 rounded-full" />
              </div>
              <div class="mt-4 pt-4 border-t border-gray-100 grid grid-cols-2 gap-4">
                <div class="text-center p-3 bg-gray-50/50 rounded-xl border border-gray-100">
                  <div class="text-[10px] text-gray-400 font-bold uppercase mb-1">活跃节点</div>
                  <div class="text-xl font-bold text-blue-600">{{ stats?.online_nodes || 0 }}</div>
                </div>
                <div class="text-center p-3 bg-gray-50/50 rounded-xl border border-gray-100">
                  <div class="text-[10px] text-gray-400 font-bold uppercase mb-1">执行总数</div>
                  <div class="text-xl font-bold text-purple-600">{{ stats?.total_events || 0 }}</div>
                </div>
              </div>
            </div>
          </template>
        </Card>
      </div>
    </div>

    <!-- Row 3: Task Monitoring -->
    <div class="grid grid-cols-12 gap-6">
      <div class="col-span-12 lg:col-span-6">
        <Card class="premium-card h-full">
          <template #title>
            <div class="flex items-center gap-2 px-2">
              <div class="w-1.5 h-6 bg-amber-500 rounded-full"></div>
              <h3 class="text-sm font-bold uppercase tracking-tight">正在运行的任务</h3>
            </div>
          </template>
          <template #content>
            <DataTable :value="runningEvents" class="nodes-table-mini" emptyMessage="当前无运行中任务">
              <Column field="id" header="Event ID" style="width: 100px">
                <template #body="{ data }">
                  <span 
                    v-tooltip.top="data.id"
                    class="event-id text-blue-500 cursor-pointer hover:underline" 
                    @click="router.push(`/logs/${data.id}`)"
                  >
                    {{ data.id.split('_').slice(-1)[0] }}
                  </span>
                </template>
              </Column>
              <Column field="job_name" header="任务名称">
                <template #body="{ data }">
                  <span class="font-bold text-blue-600 text-[12px] cursor-pointer hover:underline transition-all" @click="router.push(`/jobs/${data.job_id}/detail`)">{{ data.job_name }}</span>
                </template>
              </Column>
              <Column header="执行节点" style="width: 130px">
                <template #body="{ data }">
                  <div class="flex items-center gap-1.5 text-blue-500">
                    <i class="pi pi-desktop text-[10px]"></i>
                    <span class="text-[11px] truncate">{{ data.node_name }}</span>
                  </div>
                </template>
              </Column>
              <Column header="已耗时" style="width: 90px">
                <template #body="{ data }">
                  <span class="font-mono text-[10px] text-gray-500">
                    {{ Math.max(0, Math.floor((systemStore.currentTime - new Date(data.start_time).getTime()) / 1000)) }}s
                  </span>
                </template>
              </Column>
              <Column style="width: 40px" align="center">
                <template #body="{ data }">
                  <Button icon="pi pi-stop-circle" text severity="danger" v-tooltip.top="'中止'" @click="handleAbort(data)" style="padding: 0; width: 24px; height: 24px" />
                </template>
              </Column>
            </DataTable>
          </template>
        </Card>
      </div>

      <div class="col-span-12 lg:col-span-6">
        <Card class="premium-card h-full">
          <template #title>
            <div class="flex items-center gap-2 px-2">
              <div class="w-1.5 h-6 bg-green-500 rounded-full"></div>
              <h3 class="text-sm font-bold uppercase tracking-tight">即将运行的任务</h3>
            </div>
          </template>
          <template #content>
            <DataTable :value="upcomingJobs" class="nodes-table-mini" emptyMessage="暂无预约任务">
              <Column field="name" header="任务名称">
                <template #body="{ data }">
                  <span class="font-bold text-blue-600 text-[12px] cursor-pointer hover:underline transition-all" @click="router.push(`/jobs/${data.id}/detail`)">{{ data.name }}</span>
                </template>
              </Column>
              <Column header="下次运行" style="width: 170px">
                <template #body="{ data }">
                  <div class="flex flex-col">
                    <span class="font-mono text-[11px] text-gray-600">{{ new Date(data.next_run_time).toLocaleString('zh-CN', { hour12: false }) }}</span>
                    <span class="text-[9px] text-green-500 font-mono">{{ data.cron_expr }}</span>
                  </div>
                </template>
              </Column>
            </DataTable>
          </template>
        </Card>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dashboard {
  padding: 16px 24px 24px 24px;
  max-width: 1500px;
  margin: 0 auto;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}

.stat-card {
  border-radius: 12px;
  border: 1px solid var(--color-border-light);
  overflow: hidden;
  transition: all 0.2s ease;
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05);
}

.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.premium-card {
  border-radius: 12px;
  border: 1px solid var(--color-border-light);
  box-shadow: none;
  background: white;
}

.status-indicator {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 12px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  position: relative;
  z-index: 2;
}

.status-online { background: #22c55e; }
.status-offline { background: #94a3b8; }

.status-pulse {
  position: absolute;
  width: 16px;
  height: 16px;
  background: rgba(34, 197, 94, 0.4);
  border-radius: 50%;
  animation: pulse 2s infinite;
}

@keyframes pulse {
  0% { transform: scale(0.5); opacity: 1; }
  100% { transform: scale(2.5); opacity: 0; }
}

.hostname-text {
  font-weight: 700;
  font-size: 13px;
  color: var(--color-text-primary);
}

.event-id {
  font-family: 'JetBrains Mono', ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  font-size: 11px;
}

.nodes-table-mini :deep(.p-datatable-thead > tr > th),
.nodes-table-mini :deep(.p-datatable-tbody > tr > td) {
  padding: 12px 10px;
  font-size: 11px;
}

.usage-metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}

.mini-progress-bar {
  height: 4px !important;
  background: #f1f5f9 !important;
  border-radius: 2px;
}

.usage-label-text {
  font-family: 'JetBrains Mono', monospace;
  font-size: 9px;
  color: var(--p-surface-400);
  text-align: right;
  font-weight: 500;
}

@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .dashboard {
    padding: 12px;
  }
}
</style>
