<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { nodesApi, type Node } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import ProgressBar from 'primevue/progressbar'

const loading = ref(false)
const nodes = ref<Node[]>([])
const selectedNodes = ref<Node[]>([])
const wsStore = useWebSocketStore()


const editDialogVisible = ref(false)
const editNode = ref<Node | null>(null)
const editTags = ref<string[]>([])
const editLoading = ref(false)

const isMasterNode = (node: Node) => {
  return node.tags === 'master' || node.tags.includes('master')
}

const getMasterNodeId = () => {
  const nodesWithId = nodes.value.map(node => ({
    ...node,
    isMaster: isMasterNode(node)
  }))

  const explicitMaster = nodesWithId.find(node => node.isMaster)
  if (explicitMaster) {
    return explicitMaster.id
  }

  if (nodesWithId.length > 0) {
    const sortedByRegistration = [...nodesWithId].sort((a, b) =>
      new Date(a.registered_at).getTime() - new Date(b.registered_at).getTime()
    )
    return sortedByRegistration[0].id
  }

  return null
}

const formatUptime = (registeredAt: string) => {
  if (!registeredAt) return '-'
  const now = new Date()
  const registered = new Date(registeredAt)
  const diff = now.getTime() - registered.getTime()

  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)
  const months = Math.floor(days / 30)

  if (months > 0) return `${months} mon`
  if (days > 0) return `${days} days`
  if (hours > 0) return `${hours} hours`
  if (minutes > 0) return `${minutes} min`
  return `${seconds} sec`
}

const filteredNodes = computed(() => {
  let result = [...nodes.value]

  result.sort((a: Node, b: Node) => {
    if (a.status === 'online' && b.status !== 'online') return -1
    if (a.status !== 'online' && b.status === 'online') return 1

    const aIsMaster = isMasterNode(a)
    const bIsMaster = isMasterNode(b)
    if (aIsMaster && !bIsMaster) return -1
    if (!aIsMaster && bIsMaster) return 1

    return new Date(a.registered_at).getTime() - new Date(b.registered_at).getTime()
  })

  return result
})

const getUsageClass = (value: number) => {
  if (value < 60) return 'usage-low'
  if (value < 80) return 'usage-medium'
  return 'usage-high'
}

const getRowClass = (data: Node) => {
  if (data.status === 'offline') {
    return 'offline-row'
  }
  return ''
}

const canSelectRow = (node: Node) => {
  return !(isMasterNode(node) || node.id === getMasterNodeId())
}

const loadNodes = async () => {
  try {
    loading.value = true
    const data = await nodesApi.list()
    nodes.value = (data as any) || []
  } catch (error) {
    showToast({ severity: 'error', summary: '加载节点失败', life: 5000 })
    console.error('加载节点失败:', error)
  } finally {
    loading.value = false
  }
}

const handleDelete = async (node: Node) => {
  if (isMasterNode(node) || node.id === getMasterNodeId()) {
    showToast({ severity: 'warn', summary: '不能删除 Master 节点', life: 3000 })
    return
  }

  showConfirm({
    message: `确定要删除节点 "${node.hostname}" (${node.ip}) 吗？`,
    header: '删除 Worker 节点',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: 'danger' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        await nodesApi.delete(node.id)
        showToast({ severity: 'success', summary: '删除成功', life: 3000 })
        await loadNodes()
      } catch (error: any) {
        showToast({ severity: 'error', summary: '删除失败', detail: error.response?.data?.error || '删除失败', life: 5000 })
      }
    },
  })
}

const handleEdit = (node: Node) => {
  editNode.value = node
  try {
    editTags.value = node.tags ? JSON.parse(node.tags) : []
  } catch {
    editTags.value = node.tags ? [node.tags] : []
  }
  editDialogVisible.value = true
}

const saveEdit = async () => {
  if (!editNode.value) return

  try {
    editLoading.value = true
    await nodesApi.update(editNode.value.id, {
      tags: JSON.stringify(editTags.value)
    })
    showToast({ severity: 'success', summary: '更新成功', life: 3000 })
    editDialogVisible.value = false
    await loadNodes()
  } catch (error: any) {
    showToast({ severity: 'error', summary: '更新失败', detail: error.response?.data?.error || '更新失败', life: 5000 })
  } finally {
    editLoading.value = false
  }
}

const handleNodeStatus = (data: any) => {
  const index = nodes.value.findIndex((node) => node.id === data.node_id)
  if (index >= 0) {
    nodes.value[index] = {
      ...nodes.value[index],
      status: data.status,
      cpu_usage: data.cpu_usage,
      memory_percent: data.memory_percent,
      running_jobs: data.running_jobs,
    }
  } else {
    loadNodes()
  }
}

onMounted(async () => {
  await loadNodes()

  wsStore.onMessage('node_status', handleNodeStatus)
})

onUnmounted(() => {
  wsStore.offMessage('node_status', handleNodeStatus)
})
</script>

<template>
  <div class="workers-page">
    <!-- 概览统计 -->
    <div class="stats-grid mb-6">
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-blue-50 text-blue-500">
              <i class="pi pi-server text-xl"></i>
            </div>
            <div>
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">总节点</div>
              <div class="text-2xl font-bold">{{ nodes.length }}</div>
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
            <div>
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">在线节点</div>
              <div class="text-2xl font-bold">{{ nodes.filter(n => n.status === 'online').length }}</div>
            </div>
          </div>
        </template>
      </Card>
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-amber-50 text-amber-500">
              <i class="pi pi-bolt text-xl"></i>
            </div>
            <div>
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">运行任务</div>
              <div class="text-2xl font-bold">{{ nodes.reduce((acc, n) => acc + (n.running_jobs || 0), 0) }}</div>
            </div>
          </div>
        </template>
      </Card>
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-purple-50 text-purple-500">
              <i class="pi pi-microchip text-xl"></i>
            </div>
            <div>
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">负载均衡</div>
              <div class="text-2xl font-bold">Safe</div>
            </div>
          </div>
        </template>
      </Card>
    </div>

    <!-- 节点列表 -->
    <Card class="table-card">
      <template #content>
        <DataTable
          v-model:selection="selectedNodes"
          :value="filteredNodes"
          stripedRows
          :loading="loading"
          :rowClass="getRowClass"
          :selectableRow="canSelectRow"
          dataKey="id"
          class="workers-table"
        >
          <Column selectionMode="multiple" headerStyle="width: 3rem" />
          <Column field="hostname" header="节点信息" style="min-width: 200px">
            <template #body="{ data }">
              <div class="hostname-cell">
                <div class="status-indicator">
                  <span :class="['status-dot', data.status === 'online' ? 'status-online' : 'status-offline']"></span>
                  <span v-if="data.status === 'online'" class="status-pulse"></span>
                </div>
                <div class="flex flex-col">
                  <div class="flex items-center gap-2">
                    <span class="hostname-text">{{ data.hostname }}</span>
                    <i :class="[isMasterNode(data) ? 'pi pi-shield text-amber-500' : 'pi pi-desktop text-blue-400']" class="node-icon-mini"></i>
                  </div>
                  <span class="ip-text">{{ data.ip }}</span>
                </div>
              </div>
            </template>
          </Column>
          <Column header="PID" style="width: 80px" alignHeader="center" align="center">
            <template #body="{ data }">
              <span class="pid-text">{{ data.pid || '-' }}</span>
            </template>
          </Column>
          <Column header="角色" style="width: 110px" alignHeader="center" align="center">
            <template #body="{ data }">
              <span :class="['premium-badge', isMasterNode(data) ? 'badge-master' : 'badge-worker']">
                <i :class="isMasterNode(data) ? 'pi pi-shield' : 'pi pi-desktop'"></i>
                <span>{{ isMasterNode(data) ? 'Master' : 'Worker' }}</span>
              </span>
            </template>
          </Column>
          <Column header="并行任务" style="width: 100px" alignHeader="center" align="center">
            <template #body="{ data }">
              <span v-if="data.running_jobs > 0" class="premium-badge badge-running-mini">
                <i class="pi pi-spin pi-spinner text-[10px]"></i>
                <span>{{ data.running_jobs }}</span>
              </span>
              <span v-else class="text-gray-300">-</span>
            </template>
          </Column>
          <Column header="上线时间" style="width: 110px">
            <template #body="{ data }">
              <span class="uptime-text">{{ formatUptime(data.registered_at) }}</span>
            </template>
          </Column>
          <Column header="CPU 负载" style="width: 130px" alignHeader="center">
            <template #body="{ data }">
              <div class="usage-metric" v-if="data.status === 'online'">
                <ProgressBar :value="Math.min(data.cpu_usage || 0, 100)" :showValue="false" class="mini-progress" :class="getUsageClass(data.cpu_usage)" />
                <span class="usage-text">{{ (data.cpu_usage || 0).toFixed(1) }}%</span>
              </div>
              <span v-else class="text-gray-300">-</span>
            </template>
          </Column>
          <Column header="内存占用" style="width: 130px" alignHeader="center">
            <template #body="{ data }">
              <div class="usage-metric" v-if="data.status === 'online'">
                <ProgressBar :value="Math.min(data.memory_percent || 0, 100)" :showValue="false" class="mini-progress" :class="getUsageClass(data.memory_percent)" />
                <span class="usage-text">{{ (data.memory_percent || 0).toFixed(1) }}%</span>
              </div>
              <span v-else class="text-gray-300">-</span>
            </template>
          </Column>
          <Column header="操作" frozen alignFrozen="right" style="width: 100px">
            <template #body="{ data }">
              <div class="action-buttons">
                <Button
                  v-if="!isMasterNode(data) && data.id !== getMasterNodeId()"
                  v-tooltip.top="'编辑标签'"
                  icon="pi pi-pencil"
                  class="btn-edit"
                  @click="handleEdit(data)"
                />
                <Button
                  v-if="!isMasterNode(data) && data.id !== getMasterNodeId()"
                  v-tooltip.top="'移除节点'"
                  icon="pi pi-trash"
                  class="btn-delete"
                  @click="handleDelete(data)"
                />
              </div>
            </template>
          </Column>
        </DataTable>

        <div v-if="!loading && filteredNodes.length === 0" class="text-center py-8 text-gray-400">
          <i class="pi pi-inbox text-4xl mb-2 block"></i>
          <p>暂无节点数据</p>
        </div>
      </template>
    </Card>

    <!-- 编辑节点对话框 -->
    <Dialog v-model:visible="editDialogVisible" header="编辑节点标签" :style="{ width: '500px' }">
      <div v-if="editNode" class="flex flex-col gap-4">
        <div class="flex flex-col gap-1">
          <label class="font-medium text-sm">节点</label>
          <InputText :modelValue="editNode.hostname" disabled />
        </div>
        <div class="flex flex-col gap-1">
          <label class="font-medium text-sm">IP 地址</label>
          <InputText :modelValue="editNode.ip" disabled />
        </div>
        <div class="flex flex-col gap-1">
          <label class="font-medium text-sm">节点标签</label>
          <Select v-model="editTags" multiple filterable editable placeholder="添加标签（按回车确认）" class="w-full" />
        </div>
        <p class="text-gray-400 text-xs">节点标签用于任务调度时按标签匹配 Worker 节点</p>
      </div>
      <template #footer>
        <Button severity="secondary" @click="editDialogVisible = false" label="取消" />
        <Button severity="info" :loading="editLoading" @click="saveEdit" label="保存" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.workers-page {
  padding: 16px 24px 24px 24px;
  max-width: 1500px;
  margin: 0 auto;
}

/* Stats Cards */
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

.table-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
}

.hostname-cell {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 4px 0;
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

.status-online {
  background: #22c55e;
}

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

.status-offline {
  background: #94a3b8;
}

.node-icon-mini {
  font-size: 11px;
}

.hostname-text {
  font-weight: 700;
  font-size: 13px;
  color: var(--color-text-primary);
}

.ip-text {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  color: var(--p-surface-400);
}

.pid-text, .uptime-text {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
}

.usage-metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}

.mini-progress {
  height: 5px !important;
  background: #f1f5f9 !important;
  border-radius: 3px;
}

.usage-text {
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  color: var(--p-surface-400);
  text-align: right;
  font-weight: 500;
}

.usage-low :deep(.p-progressbar-value) { background: #10b981; }
.usage-medium :deep(.p-progressbar-value) { background: #f59e0b; }
.usage-high :deep(.p-progressbar-value) { background: #ef4444; }

/* Premium Badges */
.premium-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  border: 1px solid transparent;
}

.badge-master {
  background: #fffbeb;
  color: #b45309;
  border-color: #fde68a;
}

.badge-worker {
  background: #eff6ff;
  color: #1d4ed8;
  border-color: #dbeafe;
}

.badge-running-mini {
  background: #f0fdf4;
  color: #16a34a;
  border-color: #dcfce7;
  padding: 2px 8px;
}

.action-buttons {
  display: flex;
  gap: 6px;
}

.action-buttons :deep(.p-button) {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  transition: all 0.2s ease;
}

.btn-edit {
  background: #f8fafc !important;
  border: 1px solid #e2e8f0 !important;
  color: #475569 !important;
}

.btn-edit:hover {
  background: #f1f5f9 !important;
  border-color: #cbd5e1 !important;
  color: #0284c7 !important;
  transform: translateY(-1px);
}

.btn-delete {
  background: #fff1f2 !important;
  border: 1px solid #fecdd3 !important;
  color: #e11d48 !important;
}

.btn-delete:hover {
  background: #ffe4e6 !important;
  border-color: #fda4af !important;
  transform: translateY(-1px);
  box-shadow: 0 4px 6px -1px rgba(225, 29, 72, 0.1);
}

.workers-table :deep(.p-datatable-tbody > tr > td) {
  padding: 12px 16px;
}

:deep(.offline-row) {
  opacity: 0.6;
  filter: grayscale(0.5);
}

@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
