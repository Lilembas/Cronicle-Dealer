<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject, type Ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { statsApi, nodesApi, eventsApi, jobsApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import Card from 'primevue/card'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import { useWebSocketStore } from '@/stores/websocket'
import { useSystemStore } from '@/stores/system'
import { showConfirm, hl } from '@/utils/confirm'

interface DispatchAnimation {
  id: string // event_id
  sourceElId: string // DOM id of frozen card: frozen-${event_id}
  targetNodeId: string
  x1: number
  y1: number
  x2: number
  y2: number
  lineLength: number
  phase: 'drawing' | 'drawn' | 'fading'
}

interface FrozenCard {
  event_id: string
  job_id: string
  job_name: string
  category: string
}

const stats = ref<any>(null)
const nodes = ref<any[]>([])
const runningEvents = ref<any[]>([])
const upcomingJobs = ref<any[]>([])
const frozenCards = ref<FrozenCard[]>([])
const statsLoading = ref(false)
const nodesLoading = ref(false)
const wsStore = useWebSocketStore()
const authStore = useAuthStore()
const systemStore = useSystemStore()
const router = useRouter()
const globalRefreshHandler = inject<Ref<(() => void) | null>>('globalRefreshHandler')

const dispatchAnimations = ref<DispatchAnimation[]>([])
const highlightedEventIds = ref(new Set<string>())
const highlightedNodeIds = ref(new Set<string>())

// Continuously update line positions to follow moving cards
let rafId: number | null = null

const edgePoint = (rect: DOMRect, towardX: number, towardY: number): [number, number] => {
  const cx = rect.left + rect.width / 2
  const cy = rect.top + rect.height / 2
  const hw = rect.width / 2
  const hh = rect.height / 2
  const dx = towardX - cx
  const dy = towardY - cy
  if (dx === 0 && dy === 0) return [cx + hw, cy]
  let t: number
  if (dx === 0) t = hh / Math.abs(dy)
  else if (dy === 0) t = hw / Math.abs(dx)
  else t = Math.min(hw / Math.abs(dx), hh / Math.abs(dy))
  return [cx + dx * t, cy + dy * t]
}

const updateLinePositions = () => {
  if (dispatchAnimations.value.length === 0) {
    rafId = null
    return
  }
  for (const anim of dispatchAnimations.value) {
    const sourceEl = document.getElementById(anim.sourceElId)
    const targetEl = document.getElementById(`node-card-${anim.targetNodeId}`)
    if (!sourceEl || !targetEl) continue

    const sr = sourceEl.getBoundingClientRect()
    const tr = targetEl.getBoundingClientRect()
    const scx = sr.left + sr.width / 2, scy = sr.top + sr.height / 2
    const tcx = tr.left + tr.width / 2, tcy = tr.top + tr.height / 2
    const [x1, y1] = edgePoint(sr, tcx, tcy)
    const [x2, y2] = edgePoint(tr, scx, scy)

    anim.x1 = x1; anim.y1 = y1
    anim.x2 = x2; anim.y2 = y2
    anim.lineLength = Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2)
  }
  rafId = requestAnimationFrame(updateLinePositions)
}

const startTracking = () => {
  if (rafId === null && dispatchAnimations.value.length > 0) {
    rafId = requestAnimationFrame(updateLinePositions)
  }
}

const isManagerNode = (node: any) => {
  if (!node || !node.tags) return false
  return node.tags === 'manager' || node.tags.includes('manager')
}

const managerNodes = computed(() => sortedNodes.value.filter(n => isManagerNode(n)))
const workerNodes = computed(() => sortedNodes.value.filter(n => !isManagerNode(n)))

const canTrigger = computed(() => authStore.isAdmin || authStore.user?.role === 'user')

const sortedNodes = computed(() => {
  if (!nodes.value) return []
  return [...nodes.value].sort((a: any, b: any) => {
    const aIsManager = isManagerNode(a)
    const bIsManager = isManagerNode(b)
    if (aIsManager && !bIsManager) return -1
    if (!aIsManager && bIsManager) return 1
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
      .filter((j: any) => {
        if (!j.enabled || !j.next_run_time) return false
        const diff = new Date(j.next_run_time).getTime() - systemStore.currentTime
        return diff > 0 && diff <= 2 * 60 * 60 * 1000
      })
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

const handleTaskStatus = (data: any) => {
  if (data && data.status === 'running' && data.node_id) {
    triggerConnection(data)
  } else {
    loadData(true)
  }
}

let upcomingRefreshTimer: number | null = null

const scheduleUpcomingRefresh = () => {
  if (upcomingRefreshTimer) clearTimeout(upcomingRefreshTimer)
  upcomingRefreshTimer = window.setTimeout(() => {
    upcomingRefreshTimer = null
    loadData(true)
  }, 200)
}

const triggerConnection = (data: any) => {
  const sourceEl = document.getElementById(`upcoming-${data.job_id}`)
  const targetEl = document.getElementById(`node-card-${data.node_id}`)

  if (!sourceEl || !targetEl) {
    loadData(true)
    return
  }

  // Freeze the card data and remove from upcomingJobs immediately
  // Look up job name from upcomingJobs since backend broadcast doesn't include it
  const sourceJob = upcomingJobs.value.find(j => j.id === data.job_id)
  const frozenCard: FrozenCard = {
    event_id: data.event_id,
    job_id: data.job_id,
    job_name: sourceJob?.name || data.job_id,
    category: sourceJob?.category || ''
  }
  frozenCards.value.push(frozenCard)
  upcomingJobs.value = upcomingJobs.value.filter(j => j.id !== data.job_id)
  scheduleUpcomingRefresh()

  const sourceElId = `frozen-${data.event_id}`
  const sourceRect = sourceEl.getBoundingClientRect()
  const targetRect = targetEl.getBoundingClientRect()
  const scx = sourceRect.left + sourceRect.width / 2, scy = sourceRect.top + sourceRect.height / 2
  const tcx = targetRect.left + targetRect.width / 2, tcy = targetRect.top + targetRect.height / 2
  const [x1, y1] = edgePoint(sourceRect, tcx, tcy)
  const [x2, y2] = edgePoint(targetRect, scx, scy)
  const lineLength = Math.sqrt((x2 - x1) ** 2 + (y2 - y1) ** 2)

  const animId = data.event_id
  const nodeId = data.node_id

  dispatchAnimations.value.push({
    id: animId,
    sourceElId,
    targetNodeId: nodeId,
    x1, y1, x2, y2,
    lineLength,
    phase: 'drawing'
  })

  highlightedEventIds.value = new Set([...highlightedEventIds.value, animId])
  highlightedNodeIds.value = new Set([...highlightedNodeIds.value, nodeId])

  startTracking()

  // Phase 1: Draw line
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      const anim = dispatchAnimations.value.find(a => a.id === animId)
      if (anim) anim.phase = 'drawn'
    })
  })

  // Phase 2: Fade out
  setTimeout(() => {
    const anim = dispatchAnimations.value.find(a => a.id === animId)
    if (anim) anim.phase = 'fading'
  }, 1500)

  // Phase 3: Clean up frozen card and refresh data
  setTimeout(() => {
    dispatchAnimations.value = dispatchAnimations.value.filter(a => a.id !== animId)
    frozenCards.value = frozenCards.value.filter(c => c.event_id !== animId)
    const newEvents = new Set(highlightedEventIds.value)
    newEvents.delete(animId)
    highlightedEventIds.value = newEvents
    const newNodes = new Set(highlightedNodeIds.value)
    newNodes.delete(nodeId)
    highlightedNodeIds.value = newNodes
    loadData(true)
  }, 2500)
}

const handleAbort = (event: any) => {
  if (!canTrigger.value) return
  showConfirm({
    message: `确定要中止任务 ${hl(event.job_name)} 吗？`,
    header: '确认中止',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: 'danger' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        await eventsApi.abort(event.id)
        loadData(true)
      } catch (error) {
        console.error('中止任务失败:', error)
      }
    },
  })
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
  if (rafId !== null) cancelAnimationFrame(rafId)
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

    <!-- Row 2: Node Cards + Side Panel -->
    <div class="nodes-dispatch-layout mb-6">
      <div class="nodes-col">
        <!-- Manager Nodes -->
        <div v-if="managerNodes.length > 0" class="node-group mb-5">
          <div class="group-header">
            <div class="group-dot manager-dot"></div>
            <span class="group-title">Manager</span>
            <Tag :value="`${managerNodes.length}`" severity="warn" class="group-tag" />
          </div>
          <div class="node-cards-row">
            <div
              v-for="node in managerNodes"
              :key="node.id"
              :class="['node-card', 'manager-card', node.status !== 'online' && 'node-offline']"
            >
              <div class="nc-header">
                <div class="nc-status-wrap">
                  <span :class="['nc-status-dot', node.status === 'online' ? 'online' : 'offline']"></span>
                </div>
                <div class="nc-hostname">{{ node.hostname }}</div>
                <div class="nc-badge manager-badge">
                  <svg viewBox="0 0 24 24" fill="currentColor" class="badge-svg"><path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z"/></svg>
                  Manager
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

          <!-- Upcoming Tasks Section -->
          <div v-if="upcomingJobs.length > 0 || frozenCards.length > 0" class="upcoming-section">
            <div class="upcoming-section-header">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="upcoming-section-icon"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
              <span class="upcoming-section-title">即将安排的任务</span>
              <span class="upcoming-section-count">{{ upcomingJobs.length + frozenCards.length }}</span>
            </div>
            <div class="upcoming-cards">
              <!-- Frozen cards (dispatching) - sorted by dispatch order, first dispatched = first -->
              <div
                v-for="card in frozenCards"
                :key="card.event_id"
                :id="`frozen-${card.event_id}`"
                :class="['upcoming-task-card', highlightedEventIds.has(card.event_id) && 'dispatch-source-highlight']"
              >
                <div class="utc-header">
                  <div class="utc-dot dispatching-dot"></div>
                  <div class="utc-name" @click="router.push(`/jobs/${card.job_id}/detail`)">{{ card.job_name }}</div>
                </div>
                <div class="utc-meta">
                  <span class="utc-countdown font-mono dispatching-label">派发中</span>
                  <span v-if="card.category" class="utc-category">{{ card.category }}</span>
                </div>
              </div>
              <!-- Regular upcoming cards -->
              <div
                v-for="job in upcomingJobs"
                :key="job.id"
                :id="`upcoming-${job.id}`"
                class="upcoming-task-card"
              >
                <div class="utc-header">
                  <div class="utc-dot"></div>
                  <div class="utc-name" @click="router.push(`/jobs/${job.id}/detail`)">{{ job.name }}</div>
                </div>
                <div class="utc-meta">
                  <span class="utc-countdown font-mono">{{ formatCountdown(job.next_run_time) }}</span>
                  <span v-if="job.category" class="utc-category">{{ job.category }}</span>
                </div>
              </div>
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
              :id="`node-card-${node.id}`"
              :class="['node-card', 'worker-card', node.status !== 'online' && 'node-offline', highlightedNodeIds.has(node.id) && 'dispatch-target-highlight']"
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

              <!-- Running jobs on this node -->
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
                    <Button icon="pi pi-stop-circle" text severity="danger" v-tooltip.top="canTrigger ? '中止' : '无操作权限'" :disabled="!canTrigger" @click="handleAbort(job)" style="padding: 0; width: 18px; height: 18px; font-size: 10px;" />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Right Panel: Running Events Only -->
      <div class="tasks-col">
        <Card class="task-panel">
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
                <Button icon="pi pi-stop-circle" text severity="danger" v-tooltip.top="canTrigger ? '中止' : '无操作权限'" :disabled="!canTrigger" @click="handleAbort(event)" style="padding: 0; width: 20px; height: 20px;" />
              </div>
            </div>
          </template>
        </Card>
      </div>
    </div>

    <!-- SVG Connection Lines Overlay -->
    <Teleport to="body">
      <svg class="dispatch-svg-overlay" v-if="dispatchAnimations.length > 0">
        <defs>
          <filter id="dispatchGlow" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>
        <line
          v-for="anim in dispatchAnimations"
          :key="anim.id"
          :x1="anim.x1" :y1="anim.y1"
          :x2="anim.x2" :y2="anim.y2"
          class="dispatch-line"
          :style="{
            strokeDasharray: anim.lineLength + 'px',
            strokeDashoffset: anim.phase === 'drawing' ? anim.lineLength + 'px' : '0px',
            opacity: anim.phase === 'fading' ? 0 : 1,
          }"
          filter="url(#dispatchGlow)"
        />
      </svg>
    </Teleport>
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
.stat-label { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: #94a3b8; margin-bottom: 2px; }
.stat-value { font-size: 22px; font-weight: 800; color: #0f172a; line-height: 1.2; }
.stat-sub { font-size: 11px; color: #94a3b8; margin-top: 2px; }
.text-green { color: #10b981 !important; }
.text-amber { color: #f59e0b !important; }
.text-purple { color: #8b5cf6 !important; }
.text-red { color: #ef4444; }

/* Layout */
.nodes-dispatch-layout { display: grid; grid-template-columns: 1fr 340px; gap: 20px; align-items: start; }
.group-header { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.group-dot { width: 8px; height: 8px; border-radius: 50%; }
.manager-dot { background: #f59e0b; box-shadow: 0 0 6px #f59e0b88; }
.worker-dot { background: #3b82f6; box-shadow: 0 0 6px #3b82f688; }
.group-title { font-size: 12px; font-weight: 800; text-transform: uppercase; letter-spacing: 0.08em; color: #64748b; }
.group-tag { font-size: 11px; padding: 2px 6px; }
.node-cards-row { display: flex; flex-wrap: wrap; gap: 14px; }

/* Node Cards */
.node-card { background: white; border-radius: 14px; border: 1.5px solid #f1f5f9; padding: 14px 16px; min-width: 220px; max-width: 300px; flex: 1 1 220px; box-shadow: 0 1px 4px rgba(0,0,0,0.04); transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.3s ease, border-color 0.25s ease; position: relative; overflow: hidden; }
.manager-card { border-color: #fef3c7; background: linear-gradient(135deg, #fffbeb 0%, #fff 60%); }
.worker-card { border-color: #eff6ff; background: linear-gradient(135deg, #eff6ff 0%, #fff 60%); }
.node-card:hover { box-shadow: 0 12px 28px rgba(0,0,0,0.1); border-color: #3b82f644; }
.node-offline { opacity: 0.6; filter: grayscale(0.5); }
.nc-header { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; }
.nc-status-wrap { position: relative; width: 10px; height: 10px; flex-shrink: 0; }
.nc-status-dot { width: 8px; height: 8px; border-radius: 50%; display: block; position: relative; z-index: 2; }
.nc-status-dot.online { background: #22c55e; }
.nc-status-dot.offline { background: #94a3b8; }
.nc-hostname { font-size: 12px; font-weight: 700; color: #0f172a; flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.nc-badge { display: flex; align-items: center; gap: 3px; font-size: 11px; font-weight: 700; text-transform: uppercase; padding: 2px 7px; border-radius: 99px; letter-spacing: 0.04em; flex-shrink: 0; }
.manager-badge { background: #fef3c7; color: #92400e; }
.worker-badge { background: #dbeafe; color: #1e40af; }
.badge-svg { width: 10px; height: 10px; }

/* Metrics */
.nc-metrics { display: flex; flex-direction: column; gap: 8px; }
.metric-row { display: flex; align-items: center; gap: 8px; }
.metric-label { font-size: 11px; font-weight: 700; text-transform: uppercase; color: #94a3b8; width: 28px; flex-shrink: 0; }
.bar-container { flex: 1; height: 10px; background: #f1f5f9; border-radius: 6px; overflow: hidden; position: relative; }
.bar-fill { height: 100%; border-radius: 6px; position: relative; overflow: hidden; min-width: 4px; transition: width 0.6s cubic-bezier(0.34, 1.56, 0.64, 1); box-shadow: inset 0 1px 2px rgba(255, 255, 255, 0.2); }
.bar-fill::after { content: ''; position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: linear-gradient(180deg, rgba(255,255,255,0.15) 0%, rgba(255,255,255,0) 50%, rgba(0,0,0,0.05) 100%); }
.bar-fill.low { background: linear-gradient(135deg, #34d399 0%, #10b981 100%); }
.bar-fill.medium { background: linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%); }
.bar-fill.high { background: linear-gradient(135deg, #f87171 0%, #ef4444 100%); }
.metric-value { font-size: 11px; font-weight: 700; font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif; width: 32px; text-align: right; flex-shrink: 0; }
.metric-value.low { color: #10b981; }
.metric-value.medium { color: #f59e0b; }
.metric-value.high { color: #ef4444; }

/* Running jobs in node card */
.nc-running-jobs { margin-top: 10px; padding-top: 10px; border-top: 1px dashed #e2e8f0; display: flex; flex-direction: column; gap: 5px; }
.running-job-item { display: flex; align-items: center; gap: 8px; font-size: 11px; justify-content: space-between; }
.running-dot { width: 6px; height: 6px; border-radius: 50%; background: #22c55e; flex-shrink: 0; animation: runningPulse 1s ease-in-out infinite alternate; }
@keyframes runningPulse { from { opacity: 0.5; transform: scale(0.8); } to { opacity: 1; transform: scale(1.2); } }
.running-name-row { flex: 1; display: flex; align-items: center; gap: 4px; min-width: 0; }
.running-name { font-weight: 600; color: #3b82f6; cursor: pointer; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 120px; }
.running-name:hover { text-decoration: underline; }
.running-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }
.running-elapsed { font-family: monospace; color: #64748b; font-size: 11px; }

.task-category-tag { font-size: 11px; font-weight: 700; color: #64748b; background: #f1f5f9; padding: 0px 4px; border-radius: 4px; text-transform: uppercase; white-space: nowrap; }
.task-category-badge { font-size: 11px; font-weight: 700; color: #64748b; background: #f8fafc; border: 1px solid #e2e8f0; padding: 0px 5px; border-radius: 4px; text-transform: uppercase; margin-left: 6px; }

/* Skeleton & Empty */
.skeleton-card { min-height: 120px; }
.skeleton-line { background: #f1f5f9; border-radius: 4px; display: block; height: 12px; }
@keyframes shimmer { 0% { opacity: 0.5; } 50% { opacity: 1; } 100% { opacity: 0.5; } }
.skeleton-card { animation: shimmer 2s infinite ease-in-out; }
.empty-nodes { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 32px 0; color: #94a3b8; font-size: 12px; }
.empty-icon { width: 28px; height: 28px; opacity: 0.5; }

/* Upcoming Tasks Section (inside Manager group) */
.upcoming-section {
  margin-top: 16px;
  padding-top: 14px;
}
.upcoming-section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 10px;
}
.upcoming-section-icon {
  width: 13px;
  height: 13px;
  color: #10b981;
  flex-shrink: 0;
}
.upcoming-section-title {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: #64748b;
}
.upcoming-section-count {
  font-size: 11px;
  font-weight: 700;
  padding: 1px 7px;
  border-radius: 99px;
  background: #f0fdf4;
  color: #166534;
}
.upcoming-cards {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.upcoming-task-card {
  background: white;
  border: 1.5px solid #e2e8f0;
  border-radius: 10px;
  padding: 8px 10px;
  min-width: 130px;
  max-width: 170px;
  flex: 1 1 130px;
  transition: border-color 0.3s ease, box-shadow 0.3s ease, transform 0.2s ease;
  cursor: default;
}
.upcoming-task-card:hover {
  border-color: #10b981aa;
  box-shadow: 0 2px 8px rgba(16, 185, 129, 0.08);
}
.utc-header {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}
.utc-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: #10b981;
  flex-shrink: 0;
}
.dispatching-dot {
  background: #3b82f6;
  animation: runningPulse 1s ease-in-out infinite alternate;
}
.utc-name {
  font-size: 12px;
  font-weight: 600;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: pointer;
  transition: color 0.15s ease;
}
.utc-name:hover { color: #3b82f6; }
.utc-meta {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 3px;
  padding-left: 10px;
}
.utc-countdown {
  font-size: 11px;
  font-weight: 600;
  color: #10b981;
}
.utc-category {
  font-size: 11px;
  font-weight: 700;
  color: #64748b;
  background: #f1f5f9;
  padding: 0px 4px;
  border-radius: 3px;
  text-transform: uppercase;
}
.dispatching-label {
  color: #3b82f6 !important;
  font-weight: 700;
}

/* Right Panel - Running Events */
.task-panel { border-radius: 14px !important; border: 1px solid #f1f5f9 !important; box-shadow: 0 1px 4px rgba(0,0,0,0.04) !important; }
.panel-header { display: flex; align-items: center; gap: 8px; padding: 0 4px; }
.panel-accent { width: 4px; height: 18px; border-radius: 2px; flex-shrink: 0; }
.amber-accent { background: #f59e0b; }
.panel-title { font-size: 12px; font-weight: 800; text-transform: uppercase; letter-spacing: 0.06em; color: #0f172a; margin: 0; flex: 1; }
.panel-count { font-size: 11px; font-weight: 700; padding: 2px 8px; border-radius: 99px; }
.running-count { background: #fffbeb; color: #92400e; }
.task-list { display: flex; flex-direction: column; gap: 0; }
.task-item { display: flex; align-items: center; gap: 10px; padding: 9px 4px; border-bottom: 1px solid #f8fafc; transition: background 0.15s ease; position: relative; }
.task-item:last-child { border-bottom: none; }
.task-item:hover { background: #f8fafc; border-radius: 8px; }
.task-pulse-dot { width: 8px; height: 8px; border-radius: 50%; background: #f59e0b; flex-shrink: 0; animation: runningPulse 1s ease-in-out infinite alternate; }
.task-info { flex: 1; min-width: 0; }
.task-name-row { display: flex; align-items: center; justify-content: flex-start; width: 100%; }
.task-name { font-size: 12px; font-weight: 600; color: #3b82f6; cursor: pointer; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 180px; }
.task-name:hover { text-decoration: underline; }
.task-meta { display: flex; align-items: center; gap: 4px; margin-top: 2px; font-size: 11px; color: #94a3b8; }
.meta-icon { width: 10px; height: 10px; flex-shrink: 0; }
.font-mono { font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif; }
.empty-tasks { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 24px 0; color: #94a3b8; font-size: 12px; }
.empty-icon-sm { width: 20px; height: 20px; opacity: 0.4; margin-bottom: 2px; }

/* Dispatch Connection Line Animation */
.dispatch-svg-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  z-index: 9999;
  pointer-events: none;
}
.dispatch-line {
  stroke: #3b82f6;
  stroke-width: 2.5;
  stroke-linecap: round;
  fill: none;
  transition: stroke-dashoffset 0.6s cubic-bezier(0.25, 1, 0.5, 1), opacity 1s ease-out;
}

/* Source (upcoming task card) highlight */
.dispatch-source-highlight {
  animation: sourceBorderPulse 2.5s ease-out forwards !important;
}
@keyframes sourceBorderPulse {
  0% { border-color: #e2e8f0; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
  8% { border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12), 0 0 14px rgba(59, 130, 246, 0.2); }
  55% { border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12), 0 0 14px rgba(59, 130, 246, 0.2); }
  100% { border-color: #e2e8f0; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
}

/* Target (worker node card) highlight */
.dispatch-target-highlight {
  animation: targetBorderPulse 2.5s ease-out forwards !important;
}
@keyframes targetBorderPulse {
  0% { border-color: #eff6ff; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
  8% { border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12), 0 0 14px rgba(59, 130, 246, 0.2); }
  55% { border-color: #3b82f6; box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12), 0 0 14px rgba(59, 130, 246, 0.2); }
  100% { border-color: #eff6ff; box-shadow: 0 1px 4px rgba(0,0,0,0.04); }
}

/* Responsive */
@media (max-width: 1200px) { .nodes-dispatch-layout { grid-template-columns: 1fr; } }
@media (max-width: 1024px) { .stats-grid { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 768px) { .dashboard { padding: 12px; } .node-card { min-width: 100%; } .upcoming-task-card { min-width: 100%; max-width: 100%; } }
</style>
