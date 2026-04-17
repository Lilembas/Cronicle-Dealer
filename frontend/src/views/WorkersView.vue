<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { nodesApi, type Node } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
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
          <Column field="hostname" header="Hostname" style="min-width: 200px">
            <template #body="{ data }">
              <div class="hostname-cell">
                <span :class="['status-dot', data.status === 'online' ? 'status-online' : 'status-offline']"></span>
                <i :class="[isMasterNode(data) ? 'pi pi-shield text-amber-500' : 'pi pi-desktop text-blue-400']" class="node-icon"></i>
                <span class="hostname-text">{{ data.hostname }}</span>
              </div>
            </template>
          </Column>
          <Column field="ip" header="IP Address" style="width: 140px" />
          <Column header="PID" style="width: 80px" alignHeader="center" align="center">
            <template #body="{ data }">
              {{ data.pid || '-' }}
            </template>
          </Column>
          <Column header="Status" style="width: 100px" alignHeader="center" align="center">
            <template #body="{ data }">
              <Tag v-if="isMasterNode(data)" value="Master" severity="warn" />
              <Tag v-else value="Worker" severity="info" />
            </template>
          </Column>
          <Column header="Active Jobs" style="width: 100px" alignHeader="center" align="center">
            <template #body="{ data }">
              <Tag v-if="data.running_jobs > 0" :value="String(data.running_jobs)" severity="success" />
              <span v-else class="text-none">(None)</span>
            </template>
          </Column>
          <Column header="Uptime" style="width: 100px">
            <template #body="{ data }">
              {{ formatUptime(data.registered_at) }}
            </template>
          </Column>
          <Column header="CPU" style="width: 120px" alignHeader="center">
            <template #body="{ data }">
              <div class="usage-metric" v-if="data.status === 'online'">
                <ProgressBar :value="Math.min(data.cpu_usage || 0, 100)" :showValue="false" class="mini-progress" :class="getUsageClass(data.cpu_usage)" />
                <span class="usage-text">{{ (data.cpu_usage || 0).toFixed(1) }}%</span>
              </div>
              <span v-else>-</span>
            </template>
          </Column>
          <Column header="Memory" style="width: 120px" alignHeader="center">
            <template #body="{ data }">
              <div class="usage-metric" v-if="data.status === 'online'">
                <ProgressBar :value="Math.min(data.memory_percent || 0, 100)" :showValue="false" class="mini-progress" :class="getUsageClass(data.memory_percent)" />
                <span class="usage-text">{{ (data.memory_percent || 0).toFixed(1) }}%</span>
              </div>
              <span v-else>-</span>
            </template>
          </Column>
          <Column header="操作" frozen alignFrozen="right" style="width: 100px">
            <template #body="{ data }">
              <div class="action-row">
                <Button
                  v-if="!isMasterNode(data) && data.id !== getMasterNodeId()"
                  size="small"
                  icon="pi pi-pencil"
                  text
                  rounded
                  @click="handleEdit(data)"
                />
                <Button
                  v-if="!isMasterNode(data) && data.id !== getMasterNodeId()"
                  size="small"
                  icon="pi pi-trash"
                  severity="danger"
                  text
                  rounded
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
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.table-card {
  border-radius: 12px;
  border: 1px solid var(--color-border);
}

.hostname-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-online {
  background: #22c55e;
  box-shadow: 0 0 8px #22c55e;
}

.status-offline {
  background: #94a3b8;
}

.node-icon {
  font-size: 14px;
}

.hostname-text {
  font-weight: 600;
  font-size: 14px;
}

.usage-metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}

.mini-progress {
  height: 6px !important;
  background: #f1f5f9 !important;
}

.usage-text {
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  color: var(--p-surface-600);
  text-align: right;
}

.usage-low :deep(.p-progressbar-value) { background: #22c55e; }
.usage-medium :deep(.p-progressbar-value) { background: #f59e0b; }
.usage-high :deep(.p-progressbar-value) { background: #ef4444; }

.action-row {
  display: flex;
  gap: 4px;
}

.online-row :deep(.p-datatable-tbody > tr:hover) {
  background: #f8fafc;
}

:deep(.offline-row) {
  opacity: 0.6;
}
</style>
