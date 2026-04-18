<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject, type Ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { statsApi, nodesApi, eventsApi, jobsApi } from '@/api'
import Card from 'primevue/card'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import { useWebSocketStore } from '@/stores/websocket'
import { useSystemStore } from '@/stores/system'

const stats = ref<any>(null)
const nodes = ref<any[]>([])
const runningEvents = ref<any[]>([])
const upcomingJobs = ref<any[]>([])
const statsLoading = ref(false)
const nodesLoading = ref(false)
const wsStore = useWebSocketStore()
const systemStore = useSystemStore()
const router = useRouter()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')

const isMasterNode = (node: any) => {
  if (!node || !node.tags) return false
  return node.tags === 'master' || node.tags.includes('master')
}

const masterNodes = computed(() => sortedNodes.value.filter(n => isMasterNode(n)))
const workerNodes = computed(() => sortedNodes.value.filter(n => !isMasterNode(n)))

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

// Get running tasks for a specific node  
const getNodeRunningJobs = (nodeId: string) => {
  return runningEvents.value.filter(e => e.node_id === nodeId || e.node_name === getNodeHostname(nodeId))
}

const getNodeHostname = (nodeId: string) => {
  const node = nodes.value.find(n => n.id === nodeId)
  return node ? node.hostname : nodeId
}

const loadData = async (isBackground = false) => {
  try {
    if (!isBackground && nodes.value.length === 0) {
      statsLoading.value = true
      nodesLoading.value = true
    }

    const [statsData, nodesData, runningData, jobsData] = await Promise.all([
      statsApi.get(),
      nodesApi.list(),
      eventsApi.list({ status: 'running', page_size: 20 }),
      jobsApi.list({ page_size: 100, enabled: true })
    ])

    stats.value = statsData
    nodes.value = (nodesData as any) || []
    runningEvents.value = (runningData as any).data || []

    upcomingJobs.value = ((jobsData as any).data || [])
      .filter((j: any) => j.enabled && j.next_run_time && new Date(j.next_run_time).getTime() > systemStore.currentTime)
      .sort((a: any, b: any) => new Date(a.next_run_time).getTime() - new Date(b.next_run_time).getTime())
      .slice(0, 8)
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
    loadData(true)
  }
}

const handleTaskStatus = () => {
  loadData(true)
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
  if (percentage < 60) return 'low'
  if (percentage < 80) return 'medium'
  return 'high'
}

const formatCountdown = (nextRunTime: string) => {
  const diff = new Date(nextRunTime).getTime() - systemStore.currentTime
  if (diff <= 0) return '即将运行'
  const s = Math.floor(diff / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ${s % 60}s`
  const h = Math.floor(m / 60)
  return `${h}h ${m % 60}m`
}

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
      <div class="stat-card">
        <div class="stat-icon-wrap stat-blue">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="stat-svg"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>
        </div>
        <div class="stat-info">
          <div class="stat-label">总任务数</div>
          <div class="stat-value">{{ stats?.total_jobs || 0 }}</div>
          <div class="stat-sub">Active: <span class="text-green">{{ stats?.enabled_jobs || 0 }}</span></div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon-wrap stat-green">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="stat-svg"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>
        </div>
        <div class="stat-info">
          <div class="stat-label">成功率 (24h)</div>
          <div class="stat-value text-green">{{ stats?.total_events ? Math.round((stats.success_events / stats.total_events) * 100) : 100 }}%</div>
          <div class="stat-sub">Total: {{ stats?.total_events || 0 }}</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon-wrap stat-amber">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="stat-svg"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
        </div>
        <div class="stat-info">
          <div class="stat-label">当前运行</div>
          <div class="stat-value text-amber">{{ stats?.running_events || 0 }}</div>
          <div class="stat-sub">实时监控中</div>
        </div>
      </div>

      <div class="stat-card">
        <div class="stat-icon-wrap stat-purple">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="stat-svg"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
        </div>
        <div class="stat-info">
          <div class="stat-label">在线节点</div>
          <div class="stat-value text-purple">{{ stats?.online_nodes || 0 }}</div>
          <div class="stat-sub">Offline: <span class="text-red">{{ stats?.offline_nodes || 0 }}</span></div>
        </div>
      </div>
    </div>

    <!-- Row 2: Node Cards -->
    <div class="nodes-dispatch-layout mb-6">
      <div class="nodes-col">
        <!-- Master Nodes -->
        <div v-if="masterNodes.length > 0" class="node-group mb-5">
          <div class="group-header">
            <div class="group-dot master-dot"></div>
            <span class="group-title">Master</span>
            <Tag :value="`${masterNodes.length}`" severity="warn" class="group-tag" />
          </div>
          <div class="node-cards-row">
            <div
              v-for="node in masterNodes"
              :key="node.id"
              :class="['node-card', 'master-card', node.status !== 'online' && 'node-offline']"
            >
              <div class="nc-header">
                <div class="nc-status-wrap">
                  <span :class="['nc-status-dot', node.status === 'online' ? 'online' : 'offline']"></span>
                </div>
                <div class="nc-hostname">{{ node.hostname }}</div>
                <div class="nc-badge master-badge">
                  <svg viewBox="0 0 24 24" fill="currentColor" class="badge-svg"><path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/></svg>
                  Master
                </div>
              </div>
              <div class="nc-metrics" v-if="node.status === 'online'">
                <div class="metric-row">
                  <span class="metric-label">CPU</span>
                  <div class="bar-container">
                    <div class="bar-fill" :class="getProgressColor(node.cpu_usage || 0)" :style="{ width: Math.min(node.cpu_usage || 2, 100) + '%' }"></div>
                  </div>
                  <span class="metric-value" :class="getProgressColor(node.cpu_usage || 0)">{{ Math.round(node.cpu_usage || 0) }}%</span>
                </div>
                <div class="metric-row">
                  <span class="metric-label">MEM</span>
                  <div class="bar-container">
                    <div class="bar-fill" :class="getProgressColor(node.memory_percent || 0)" :style="{ width: Math.min(node.memory_percent || 2, 100) + '%' }"></div>
                  </div>
                  <span class="metric-value" :class="getProgressColor(node.memory_percent || 0)">{{ Math.round(node.memory_percent || 0) }}%</span>
                </div>
              </div>
              <div class="nc-offline-msg" v-else>节点离线</div>
            </div>
          </div>
        </div>

        <!-- Worker Nodes -->
        <div class="node-group">
          <div class="group-header">
            <div class="group-dot worker-dot"></div>
            <span class="group-title">Workers</span>
            <Tag :value="`${workerNodes.length}`" severity="info" class="group-tag" />
          </div>

          <div v-if="workerNodes.length === 0 && (nodesLoading || nodes.length === 0)" class="node-cards-row">
            <div v-for="i in 3" :key="i" class="node-card skeleton-card">
              <div class="skeleton-line w-60 h-3 mb-2"></div>
              <div class="skeleton-line w-full h-2 mb-1"></div>
              <div class="skeleton-line w-full h-2"></div>
            </div>
          </div>

          <div v-else-if="workerNodes.length === 0" class="empty-nodes">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="empty-icon"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
            <span>暂无 Worker 节点</span>
          </div>

          <div v-else class="node-cards-row">
            <div
              v-for="node in workerNodes"
              :key="node.id"
              :class="['node-card', 'worker-card', node.status !== 'online' && 'node-offline']"
            >
              <div class="nc-header">
                <div class="nc-status-wrap">
                  <span :class="['nc-status-dot', node.status === 'online' ? 'online' : 'offline']"></span>
                </div>
                <div class="nc-hostname">{{ node.hostname }}</div>
                <div class="nc-badge worker-badge">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="badge-svg"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
                  Worker
                </div>
              </div>
              <div class="nc-metrics" v-if="node.status === 'online'">
                <div class="metric-row">
                  <span class="metric-label">CPU</span>
                  <div class="bar-container">
                    <div class="bar-fill" :class="getProgressColor(node.cpu_usage || 0)" :style="{ width: Math.min(node.cpu_usage || 2, 100) + '%' }"></div>
                  </div>
                  <span class="metric-value" :class="getProgressColor(node.cpu_usage || 0)">{{ Math.round(node.cpu_usage || 0) }}%</span>
                </div>
                <div class="metric-row">
                  <span class="metric-label">MEM</span>
                  <div class="bar-container">
                    <div class="bar-fill" :class="getProgressColor(node.memory_percent || 0)" :style="{ width: Math.min(node.memory_percent || 2, 100) + '%' }"></div>
                  </div>
                  <span class="metric-value" :class="getProgressColor(node.memory_percent || 0)">{{ Math.round(node.memory_percent || 0) }}%</span>
                </div>
              </div>
              <div class="nc-offline-msg" v-else>节点离线</div>

              <!-- Back to simple list style for card tasks -->
              <div class="nc-running-jobs" v-if="getNodeRunningJobs(node.id).length > 0">
                <div
                  class="running-job-item"
                  v-for="job in getNodeRunningJobs(node.id)"
                  :key="job.id"
                >
                  <span class="running-dot"></span>
                  <div class="running-name-row">
                    <span class="running-name" @click="router.push(`/logs/${job.id}`)">{{ job.job_name }}</span>
                    <span v-if="job.job_category" class="task-category-tag">{{ job.job_category }}</span>
                  </div>
                  <div class="running-actions">
                    <span class="running-elapsed">{{ Math.max(0, Math.floor((systemStore.currentTime - new Date(job.start_time).getTime()) / 1000)) }}s</span>
                    <Button icon="pi pi-stop-circle" text severity="danger" v-tooltip.top="'中止'" @click="handleAbort(job)" style="padding: 0; width: 18px; height: 18px; font-size: 10px;" />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Right Panel -->
      <div class="tasks-col">
        <Card class="task-panel mb-4">
          <template #title>
            <div class="panel-header">
              <div class="panel-accent amber-accent"></div>
              <h3 class="panel-title">正在运行</h3>
              <span class="panel-count running-count">{{ runningEvents.length }}</span>
            </div>
          </template>
          <template #content>
            <div v-if="runningEvents.length === 0" class="empty-tasks">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="empty-icon-sm"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
              <span>当前无运行中任务</span>
            </div>
            <div v-else class="task-list">
              <div v-for="event in runningEvents" :key="event.id" class="task-item running-item">
                <div class="task-pulse-dot"></div>
                <div class="task-info">
                  <div class="task-name-row">
                    <div class="task-name" @click="router.push(`/logs/${event.id}`)">{{ event.job_name }}</div>
                    <span v-if="event.job_category" class="task-category-badge">{{ event.job_category }}</span>
                  </div>
                  <div class="task-meta">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="meta-icon"><rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21"/><line x1="12" y1="17" x2="12" y2="21"/></svg>
                    <span>{{ event.node_name }}</span>
                    <span class="sep">·</span>
                    <span class="font-mono">{{ Math.max(0, Math.floor((systemStore.currentTime - new Date(event.start_time).getTime()) / 1000)) }}s</span>
                  </div>
                </div>
                <Button icon="pi pi-stop-circle" text severity="danger" v-tooltip.top="'中止'" @click="handleAbort(event)" style="padding: 0; width: 20px; height: 20px;" />
              </div>
            </div>
          </template>
        </Card>

        <Card class="task-panel">
          <template #title>
            <div class="panel-header">
              <div class="panel-accent green-accent"></div>
              <h3 class="panel-title">即将运行</h3>
              <span class="panel-count upcoming-count">{{ upcomingJobs.length }}</span>
            </div>
          </template>
          <template #content>
            <div v-if="upcomingJobs.length === 0" class="empty-tasks">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="empty-icon-sm"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"/><line x1="16" y1="2" x2="16" y2="6"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="3" y1="10" x2="21" y2="10"/></svg>
              <span>暂无待运行任务</span>
            </div>
            <div v-else class="task-list">
              <div v-for="job in upcomingJobs" :key="job.id" class="task-item upcoming-item">
                <div class="upcoming-dot"></div>
                <div class="task-info">
                  <div class="task-name-row">
                    <div class="task-name" @click="router.push(`/jobs/${job.id}/detail`)">{{ job.name }}</div>
                    <span v-if="job.category" class="task-category-badge">{{ job.category }}</span>
                  </div>
                  <div class="task-meta">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="meta-icon"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                    <span class="font-mono countdown">{{ formatCountdown(job.next_run_time) }}</span>
                    <span class="sep">·</span>
                    <span class="cron-text">{{ job.cron_expr }}</span>
                  </div>
                </div>
              </div>
            </div>
          </template>
        </Card>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dashboard { padding: 16px 24px 24px 24px; max-width: 1600px; margin: 0 auto; }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; }
.stat-card { background: white; border-radius: 14px; border: 1px solid #f1f5f9; padding: 16px 18px; display: flex; align-items: center; gap: 14px; box-shadow: 0 1px 3px rgba(0,0,0,0.04); transition: transform 0.2s ease, box-shadow 0.2s ease; }
.stat-card:hover { transform: translateY(-2px); box-shadow: 0 8px 20px rgba(0,0,0,0.06); }
.stat-icon-wrap { width: 44px; height: 44px; border-radius: 12px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.stat-svg { width: 20px; height: 20px; }
.stat-blue { background: #eff6ff; color: #3b82f6; }
.stat-green { background: #f0fdf4; color: #10b981; }
.stat-amber { background: #fffbeb; color: #f59e0b; }
.stat-purple { background: #f5f3ff; color: #8b5cf6; }
.stat-label { font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: #94a3b8; margin-bottom: 2px; }
.stat-value { font-size: 22px; font-weight: 800; color: #0f172a; line-height: 1.2; }
.stat-sub { font-size: 10px; color: #94a3b8; margin-top: 2px; }
.text-green { color: #10b981 !important; }
.text-amber { color: #f59e0b !important; }
.text-purple { color: #8b5cf6 !important; }
.text-red { color: #ef4444; }

.nodes-dispatch-layout { display: grid; grid-template-columns: 1fr 380px; gap: 20px; align-items: start; }
.group-header { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.group-dot { width: 8px; height: 8px; border-radius: 50%; }
.master-dot { background: #f59e0b; box-shadow: 0 0 6px #f59e0b88; }
.worker-dot { background: #3b82f6; box-shadow: 0 0 6px #3b82f688; }
.group-title { font-size: 11px; font-weight: 800; text-transform: uppercase; letter-spacing: 0.08em; color: #64748b; }
.group-tag { font-size: 10px; padding: 2px 6px; }
.node-cards-row { display: flex; flex-wrap: wrap; gap: 14px; }
.node-card { background: white; border-radius: 14px; border: 1.5px solid #f1f5f9; padding: 14px 16px; min-width: 220px; max-width: 300px; flex: 1 1 220px; box-shadow: 0 1px 4px rgba(0,0,0,0.04); transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.3s ease, border-color 0.25s ease; position: relative; overflow: hidden; }
.master-card { border-color: #fef3c7; background: linear-gradient(135deg, #fffbeb 0%, #fff 60%); }
.worker-card { border-color: #eff6ff; background: linear-gradient(135deg, #eff6ff 0%, #fff 60%); }
.node-card:hover { box-shadow: 0 12px 28px rgba(0,0,0,0.1); border-color: #3b82f644; }
.node-offline { opacity: 0.6; filter: grayscale(0.5); }
.nc-header { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.nc-status-wrap { position: relative; width: 10px; height: 10px; flex-shrink: 0; }
.nc-status-dot { width: 8px; height: 8px; border-radius: 50%; display: block; position: relative; z-index: 2; }
.nc-status-dot.online { background: #22c55e; }
.nc-status-dot.offline { background: #94a3b8; }
.nc-hostname { font-size: 12px; font-weight: 700; color: #0f172a; flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.nc-badge { display: flex; align-items: center; gap: 3px; font-size: 9px; font-weight: 700; text-transform: uppercase; padding: 2px 7px; border-radius: 99px; letter-spacing: 0.04em; flex-shrink: 0; }
.master-badge { background: #fef3c7; color: #92400e; }
.worker-badge { background: #dbeafe; color: #1e40af; }
.badge-svg { width: 10px; height: 10px; }

.nc-metrics { display: flex; flex-direction: column; gap: 8px; }
.metric-row { display: flex; align-items: center; gap: 8px; }
.metric-label { font-size: 9px; font-weight: 700; text-transform: uppercase; color: #94a3b8; width: 28px; flex-shrink: 0; }
.bar-container { flex: 1; height: 10px; background: #f1f5f9; border-radius: 6px; overflow: hidden; position: relative; }
.bar-fill { height: 100%; border-radius: 6px; position: relative; overflow: hidden; min-width: 4px; transition: width 0.6s cubic-bezier(0.34, 1.56, 0.64, 1); box-shadow: inset 0 1px 2px rgba(255, 255, 255, 0.2); }
.bar-fill::after { content: ''; position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: linear-gradient(180deg, rgba(255,255,255,0.15) 0%, rgba(255,255,255,0) 50%, rgba(0,0,0,0.05) 100%); }
.bar-fill.low { background: linear-gradient(135deg, #34d399 0%, #10b981 100%); }
.bar-fill.medium { background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%); }
.bar-fill.high { background: linear-gradient(135deg, #f87171 0%, #ef4444 100%); }
.metric-value { font-size: 10px; font-weight: 700; font-family: 'JetBrains Mono', monospace; width: 32px; text-align: right; flex-shrink: 0; }
.metric-value.low { color: #10b981; }
.metric-value.medium { color: #f59e0b; }
.metric-value.high { color: #ef4444; }

.nc-running-jobs { margin-top: 10px; padding-top: 10px; border-top: 1px dashed #e2e8f0; display: flex; flex-direction: column; gap: 5px; }
.running-job-item { display: flex; align-items: center; gap: 8px; font-size: 10px; justify-content: space-between; }
.running-dot { width: 6px; height: 6px; border-radius: 50%; background: #22c55e; flex-shrink: 0; animation: runningPulse 1s ease-in-out infinite alternate; }
@keyframes runningPulse { from { opacity: 0.5; transform: scale(0.8); } to { opacity: 1; transform: scale(1.2); } }
.running-name-row { flex: 1; display: flex; align-items: center; gap: 4px; min-width: 0; }
.running-name { font-weight: 600; color: #3b82f6; cursor: pointer; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 120px; }
.running-name:hover { text-decoration: underline; }
.running-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.running-elapsed { font-family: monospace; color: #64748b; font-size: 9px; }

.task-category-tag { font-size: 8px; font-weight: 700; color: #64748b; background: #f1f5f9; padding: 0px 4px; border-radius: 4px; text-transform: uppercase; white-space: nowrap; }
.task-category-badge { font-size: 9px; font-weight: 700; color: #64748b; background: #f8fafc; border: 1px solid #e2e8f0; padding: 0px 5px; border-radius: 4px; text-transform: uppercase; margin-left: 6px; }

.skeleton-card { min-height: 120px; }
.skeleton-line { background: #f1f5f9; border-radius: 4px; display: block; height: 12px; }
@keyframes shimmer { 0% { opacity: 0.5; } 50% { opacity: 1; } 100% { opacity: 0.5; } }
.skeleton-card { animation: shimmer 2s infinite ease-in-out; }
.empty-nodes { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 32px 0; color: #94a3b8; font-size: 12px; }
.empty-icon { width: 28px; height: 28px; opacity: 0.5; }

.task-panel { border-radius: 14px !important; border: 1px solid #f1f5f9 !important; box-shadow: 0 1px 4px rgba(0,0,0,0.04) !important; }
.panel-header { display: flex; align-items: center; gap: 8px; padding: 0 4px; }
.panel-accent { width: 4px; height: 18px; border-radius: 2px; flex-shrink: 0; }
.amber-accent { background: #f59e0b; }
.green-accent { background: #10b981; }
.panel-title { font-size: 12px; font-weight: 800; text-transform: uppercase; letter-spacing: 0.06em; color: #0f172a; margin: 0; flex: 1; }
.panel-count { font-size: 10px; font-weight: 700; padding: 2px 8px; border-radius: 99px; }
.running-count { background: #fffbeb; color: #92400e; }
.upcoming-count { background: #f0fdf4; color: #166534; }
.task-list { display: flex; flex-direction: column; gap: 0; }
.task-item { display: flex; align-items: center; gap: 10px; padding: 9px 4px; border-bottom: 1px solid #f8fafc; transition: background 0.15s ease; position: relative; }
.task-item:last-child { border-bottom: none; }
.task-item:hover { background: #f8fafc; border-radius: 8px; }
.task-pulse-dot { width: 8px; height: 8px; border-radius: 50%; background: #f59e0b; flex-shrink: 0; animation: runningPulse 1s ease-in-out infinite alternate; }
.upcoming-dot { width: 6px; height: 6px; border-radius: 50%; background: #10b981; flex-shrink: 0; }
.task-info { flex: 1; min-width: 0; }
.task-name-row { display: flex; align-items: center; justify-content: flex-start; width: 100%; }
.task-name { font-size: 12px; font-weight: 600; color: #3b82f6; cursor: pointer; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 180px; }
.task-name:hover { text-decoration: underline; }
.task-meta { display: flex; align-items: center; gap: 4px; margin-top: 2px; font-size: 10px; color: #94a3b8; }
.meta-icon { width: 10px; height: 10px; flex-shrink: 0; }
.font-mono { font-family: 'JetBrains Mono', monospace; }
.countdown { color: #10b981; font-weight: 600; }
.cron-text { font-family: 'JetBrains Mono', monospace; color: #94a3b8; font-size: 9px; }
.empty-tasks { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 24px 0; color: #94a3b8; font-size: 11px; }
.empty-icon-sm { width: 20px; height: 20px; opacity: 0.4; margin-bottom: 2px; }

@media (max-width: 1200px) { .nodes-dispatch-layout { grid-template-columns: 1fr; } .tasks-col { flex-direction: row; gap: 12px; } .task-panel { flex: 1; } }
@media (max-width: 1024px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 768px) { .dashboard { padding: 12px; } .tasks-col { flex-direction: column; } .node-card { min-width: 100%; } }
</style>
