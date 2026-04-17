<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { statsApi, nodesApi } from '@/api'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import ProgressBar from 'primevue/progressbar'
import Card from 'primevue/card'
import ProgressSpinner from 'primevue/progressspinner'
import { useWebSocketStore } from '@/stores/websocket'

const stats = ref<any>(null)
const nodes = ref<any[]>([])
const statsLoading = ref(true)
const nodesLoading = ref(true)
const wsStore = useWebSocketStore()

const loadData = async () => {
  try {
    statsLoading.value = true
    nodesLoading.value = true

    const [statsData, nodesData] = await Promise.all([
      statsApi.get(),
      nodesApi.list()
    ])

    stats.value = statsData
    nodes.value = nodesData
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

const handleTaskStatus = (data: any) => {
  statsApi.get().then(statsData => {
    stats.value = statsData
  })
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

onMounted(async () => {
  await loadData()

  wsStore.onMessage('task_status', handleTaskStatus)
  wsStore.onMessage('node_status', handleNodeStatus)
})

onUnmounted(() => {
  wsStore.offMessage('task_status', handleTaskStatus)
  wsStore.offMessage('node_status', handleNodeStatus)
})
</script>

<template>
  <div class="dashboard">
    <!-- 页面操作 -->
    <div class="page-header">
      <Button
        severity="info"
        icon="pi pi-refresh"
        @click="refetchStats()"
        class="refresh-btn"
        label="刷新"
      />
    </div>

    <!-- 统计卡片网格 -->
    <div class="relative">
      <ProgressSpinner v-if="statsLoading" style="width:40px;height:40px" strokeWidth="4" class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-10" />
      <div class="stats-grid" :class="{ 'opacity-50 pointer-events-none': statsLoading }">
        <!-- 总任务数 -->
        <div class="stat-card">
          <div class="stat-card-inner">
            <div class="stat-icon stat-icon-blue">
              <i class="pi pi-clock" style="font-size: 28px"></i>
            </div>
            <div class="stat-content">
              <div class="stat-label">总任务数</div>
              <div class="stat-value">{{ stats?.total_jobs || 0 }}</div>
              <div class="stat-sub">已启用: {{ stats?.enabled_jobs || 0 }}</div>
            </div>
          </div>
        </div>

        <!-- 成功执行 -->
        <div class="stat-card">
          <div class="stat-card-inner">
            <div class="stat-icon stat-icon-green">
              <i class="pi pi-check-circle" style="font-size: 28px"></i>
            </div>
            <div class="stat-content">
              <div class="stat-label">成功执行</div>
              <div class="stat-value stat-value-success">{{ stats?.success_events || 0 }}</div>
              <div class="stat-sub">总执行: {{ stats?.total_events || 0 }}</div>
            </div>
          </div>
        </div>

        <!-- 失败执行 -->
        <div class="stat-card">
          <div class="stat-card-inner">
            <div class="stat-icon stat-icon-red">
              <i class="pi pi-times-circle" style="font-size: 28px"></i>
            </div>
            <div class="stat-content">
              <div class="stat-label">失败执行</div>
              <div class="stat-value stat-value-failed">{{ stats?.failed_events || 0 }}</div>
              <div class="stat-sub">运行中: {{ stats?.running_events || 0 }}</div>
            </div>
          </div>
        </div>

        <!-- 在线节点 -->
        <div class="stat-card">
          <div class="stat-card-inner">
            <div class="stat-icon stat-icon-purple">
              <i class="pi pi-desktop" style="font-size: 28px"></i>
            </div>
            <div class="stat-content">
              <div class="stat-label">在线节点</div>
              <div class="stat-value stat-value-nodes">{{ stats?.online_nodes || 0 }}</div>
              <div class="stat-sub">离线: {{ stats?.offline_nodes || 0 }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 节点状态列表 -->
    <div class="nodes-section">
      <Card class="nodes-card">
        <template #title>
          <div class="card-header">
            <h3 class="card-title">节点状态</h3>
            <Tag :value="`${nodes?.length || 0} 个节点`" severity="info" />
          </div>
        </template>
        <template #content>
          <DataTable
            :value="nodes || []"
            stripedRows
            class="nodes-table"
            :loading="nodesLoading"
            emptyMessage="暂无节点数据"
          >
            <Column field="hostname" header="主机名" style="min-width: 160px">
              <template #body="{ data }">
                <span class="truncate block">{{ data.hostname }}</span>
              </template>
            </Column>
            <Column field="ip" header="IP 地址" style="width: 140px" />
            <Column header="状态" style="width: 100px" alignHeader="center" align="center">
              <template #body="{ data }">
                <Tag :value="data.status === 'online' ? '在线' : '离线'" :severity="data.status === 'online' ? 'success' : 'secondary'" />
              </template>
            </Column>
            <Column header="CPU 使用率" style="width: 160px">
              <template #body="{ data }">
                <div class="progress-cell">
                  <ProgressBar
                    :value="Math.round(data.cpu_usage)"
                    :showValue="false"
                    style="height: 6px"
                    :style="{ '--progress-color': getProgressColor(data.cpu_usage) }"
                  />
                </div>
              </template>
            </Column>
            <Column header="内存使用率" style="width: 160px">
              <template #body="{ data }">
                <div class="progress-cell">
                  <ProgressBar
                    :value="Math.round(data.memory_percent)"
                    :showValue="false"
                    style="height: 6px"
                    :style="{ '--progress-color': getProgressColor(data.memory_percent) }"
                  />
                </div>
              </template>
            </Column>
            <Column header="磁盘使用率" style="width: 160px">
              <template #body="{ data }">
                <div class="progress-cell">
                  <ProgressBar
                    :value="Math.round(data.disk_percent)"
                    :showValue="false"
                    style="height: 6px"
                    :style="{ '--progress-color': getProgressColor(data.disk_percent) }"
                  />
                </div>
              </template>
            </Column>
            <Column field="running_jobs" header="运行任务" style="width: 100px" alignHeader="center" align="center" />
            <Column field="version" header="版本" style="width: 100px" />
          </DataTable>
        </template>
      </Card>
    </div>
  </div>
</template>

<style scoped>
.dashboard {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  margin-bottom: 24px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: var(--color-surface);
  border-radius: 12px;
  border: 1px solid var(--color-border);
  overflow: hidden;
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
  cursor: pointer;
}

.stat-card:hover {
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  border-color: #cbd5e1;
}

.stat-card-inner {
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  flex-shrink: 0;
}

.stat-icon-blue { background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%); }
.stat-icon-green { background: linear-gradient(135deg, #10b981 0%, #059669 100%); }
.stat-icon-red { background: linear-gradient(135deg, #ef4444 0%, #dc2626 100%); }
.stat-icon-purple { background: linear-gradient(135deg, #8b5cf6 0%, #7c3aed 100%); }

.stat-content {
  flex: 1;
  min-width: 0;
}

.stat-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-muted);
  margin-bottom: 4px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--color-text-primary);
  line-height: 1.2;
  margin-bottom: 2px;
}

.stat-value-success { color: #10b981; }
.stat-value-failed { color: #ef4444; }
.stat-value-nodes { color: #8b5cf6; }

.stat-sub {
  font-size: 12px;
  color: var(--color-text-muted);
}

.nodes-section {
  margin-top: 0;
}

.nodes-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0;
}

.nodes-table {
  width: 100%;
}

.progress-cell {
  padding: 0 8px;
}

@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .dashboard {
    padding: 16px;
  }

  .stats-grid {
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }

  .stat-card-inner {
    padding: 16px;
  }

  .stat-value {
    font-size: 24px;
  }
}

@media (max-width: 480px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .nodes-table :deep(.p-datatable-wrapper) {
    overflow-x: auto;
  }
}
</style>
