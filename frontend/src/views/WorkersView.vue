<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { nodesApi, strategiesApi, type Node, type LoadBalanceStrategy, type LBFormulaMetric, type FormulaParameter } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { useAuthStore } from '@/stores/auth'
import { showToast } from '@/utils/toast'
import { showConfirm, hl } from '@/utils/confirm'
import Button from 'primevue/button'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Chips from 'primevue/chips'
import InputNumber from 'primevue/inputnumber'
import ProgressBar from 'primevue/progressbar'

const loading = ref(false)
const nodes = ref<Node[]>([])
const wsStore = useWebSocketStore()
const authStore = useAuthStore()

const editDialogVisible = ref(false)
const editNode = ref<Node | null>(null)
const editTags = ref<string[]>([])
const editMaxConcurrent = ref(0)
const editLoading = ref(false)

const strategies = ref<LoadBalanceStrategy[]>([])
const strategyDialogVisible = ref(false)
const strategyFormVisible = ref(false)
const strategyLoading = ref(false)
const formulaParams = ref<FormulaParameter[]>([])
const formulaParamVisible = ref(false)

const currentStrategy = ref<{
  id?: string
  name: string
  description: string
  direction: string
  metrics: LBFormulaMetric[]
}>({
  name: '',
  description: '',
  direction: 'asc',
  metrics: [],
})

const formulaValidation = ref<Record<string, { valid: boolean; error?: string }>>({})
let validateTimer: ReturnType<typeof setTimeout> | null = null

const isEditingStrategy = computed(() => !!currentStrategy.value.id)

function isManagerNode(node: Node): boolean {
  return node.tags?.includes('manager') ?? false
}

function isRemovableNode(node: Node): boolean {
  return !isManagerNode(node) && node.id !== getManagerNodeId()
}

function getManagerNodeId(): string | null {
  const manager = nodes.value.find(node => isManagerNode(node))
  if (manager) return manager.id

  if (nodes.value.length > 0) {
    const sorted = [...nodes.value].sort((a, b) =>
      new Date(a.registered_at).getTime() - new Date(b.registered_at).getTime()
    )
    return sorted[0].id
  }

  return null
}

function formatUptime(registeredAt: string): string {
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
  return [...nodes.value].sort((a: Node, b: Node) => {
    if (a.status === 'online' && b.status !== 'online') return -1
    if (a.status !== 'online' && b.status === 'online') return 1

    const aIsManager = isManagerNode(a)
    const bIsManager = isManagerNode(b)
    if (aIsManager && !bIsManager) return -1
    if (!aIsManager && bIsManager) return 1

    return new Date(a.registered_at).getTime() - new Date(b.registered_at).getTime()
  })
})

const getUsageClass = (value: number) => {
  if (value < 60) return 'usage-low'
  if (value < 80) return 'usage-medium'
  return 'usage-high'
}

const getRowClass = (data: Node) => {
  return data.status === 'offline' ? 'offline-row' : ''
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
  if (!isRemovableNode(node)) {
    showToast({ severity: 'warn', summary: '不能删除 Manager 节点', life: 3000 })
    return
  }

  showConfirm({
    message: `确定要删除节点 ${hl(node.hostname)} (${hl(node.ip)}) 吗？`,
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
  editMaxConcurrent.value = node.max_concurrent || 0
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
      tags: JSON.stringify(editTags.value),
      max_concurrent: editMaxConcurrent.value
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

const loadStrategies = async () => {
  try {
    const data = await strategiesApi.list()
    strategies.value = (data as any) || []
  } catch (error) {
    console.error('加载策略失败:', error)
  }
}

const loadFormulaParams = async () => {
  try {
    const data = await strategiesApi.getParameters()
    formulaParams.value = (data as any).parameters || []
  } catch {
    formulaParams.value = []
  }
}

const openStrategyDialog = async () => {
  await loadStrategies()
  await loadFormulaParams()
  strategyDialogVisible.value = true
}

const openStrategyForm = (strategy?: LoadBalanceStrategy) => {
  formulaValidation.value = {}
  if (strategy) {
    let metrics: LBFormulaMetric[] = []
    try { metrics = JSON.parse(strategy.metrics) } catch { metrics = [] }
    currentStrategy.value = {
      id: strategy.id,
      name: strategy.name,
      description: strategy.description || '',
      direction: strategy.direction || 'asc',
      metrics,
    }
  } else {
    currentStrategy.value = {
      name: '',
      description: '',
      direction: 'asc',
      metrics: [],
    }
  }
  strategyFormVisible.value = true
}

const addMetric = () => {
  currentStrategy.value.metrics.push({
    id: `m_${Date.now()}_${Math.random().toString(36).slice(2, 6)}`,
    name: '',
    formula: '',
    weight: 1.0,
    description: '',
  })
}

const removeMetric = (index: number) => {
  const metricId = currentStrategy.value.metrics[index]?.id
  currentStrategy.value.metrics.splice(index, 1)
  if (metricId) {
    delete formulaValidation.value[metricId]
  }
}

const validateFormulaDebounced = (metric: LBFormulaMetric) => {
  if (validateTimer) clearTimeout(validateTimer)
  if (!metric.formula.trim()) {
    delete formulaValidation.value[metric.id]
    return
  }
  validateTimer = setTimeout(async () => {
    try {
      const resp = await strategiesApi.validate(metric.formula)
      const data = (resp as any).data || resp
      formulaValidation.value[metric.id] = { valid: data.valid, error: data.error }
    } catch {
      formulaValidation.value[metric.id] = { valid: false, error: '验证请求失败' }
    }
  }, 400)
}

const saveStrategy = async () => {
  if (!currentStrategy.value.name.trim()) {
    showToast({ severity: 'warn', summary: '请输入策略名称', life: 3000 })
    return
  }

  const hasInvalid = currentStrategy.value.metrics.some(m => {
    if (!m.formula.trim()) return false
    return formulaValidation.value[m.id]?.valid === false
  })
  if (hasInvalid) {
    showToast({ severity: 'warn', summary: '存在无效的公式，请修正', life: 3000 })
    return
  }

  try {
    strategyLoading.value = true
    const payload: any = {
      name: currentStrategy.value.name,
      description: currentStrategy.value.description,
      direction: currentStrategy.value.direction,
      metrics: currentStrategy.value.metrics,
    }
    if (isEditingStrategy.value) {
      await strategiesApi.update(currentStrategy.value.id!, payload)
    } else {
      await strategiesApi.create(payload)
    }
    showToast({ severity: 'success', summary: isEditingStrategy.value ? '更新成功' : '创建成功', life: 3000 })
    strategyFormVisible.value = false
    await loadStrategies()
  } catch (error: any) {
    showToast({ severity: 'error', summary: '操作失败', detail: error.response?.data?.error || '操作失败', life: 5000 })
  } finally {
    strategyLoading.value = false
  }
}

const deleteStrategy = async (strategy: LoadBalanceStrategy) => {
  showConfirm({
    message: `确定要删除策略 ${hl(strategy.name)} 吗？`,
    header: '删除策略',
    icon: 'pi pi-exclamation-triangle',
    acceptProps: { label: '确定', severity: 'danger' },
    rejectProps: { label: '取消', severity: 'secondary', outlined: true },
    accept: async () => {
      try {
        await strategiesApi.delete(strategy.id)
        showToast({ severity: 'success', summary: '删除成功', life: 3000 })
        await loadStrategies()
      } catch (error: any) {
        showToast({ severity: 'error', summary: '删除失败', detail: error.response?.data?.error || '删除失败', life: 5000 })
      }
    },
  })
}

const getMetricCount = (strategy: LoadBalanceStrategy) => {
  try {
    const metrics = JSON.parse(strategy.metrics)
    return metrics.length
  } catch {
    return 0
  }
}

const insertParam = (param: FormulaParameter) => {
  const lastMetric = currentStrategy.value.metrics[currentStrategy.value.metrics.length - 1]
  if (lastMetric) {
    lastMetric.formula = lastMetric.formula ? `${lastMetric.formula} + ${param.name}` : param.name
    validateFormulaDebounced(lastMetric)
  }
}

onMounted(async () => {
  await loadNodes()
  await loadStrategies()
  wsStore.onMessage('node_status', handleNodeStatus)
})

onUnmounted(() => {
  wsStore.offMessage('node_status', handleNodeStatus)
})
</script>

<template>
  <div class="workers page-container">
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
      <Card class="stat-card stat-card-clickable" @click="openStrategyDialog">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-purple-50 text-purple-500">
              <i class="pi pi-share-alt text-xl"></i>
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">均衡策略</div>
              <div class="text-2xl font-bold">{{ strategies.length || 0 }}</div>
            </div>
            <Button icon="pi pi-cog" text rounded severity="secondary" class="text-xs" v-tooltip.top="'管理策略'" />
          </div>
        </template>
      </Card>
    </div>

    <Card class="table-card">
      <template #content>
        <DataTable
          :value="filteredNodes"
          stripedRows
          :loading="loading"
          :rowClass="getRowClass"
          dataKey="id"
          class="workers-table"
        >
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
                    <i :class="[isManagerNode(data) ? 'pi pi-shield text-amber-500' : 'pi pi-desktop text-blue-400']" class="node-icon-mini"></i>
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
              <span :class="['premium-badge', isManagerNode(data) ? 'badge-manager' : 'badge-worker']">
                <i :class="isManagerNode(data) ? 'pi pi-shield' : 'pi pi-desktop'"></i>
                <span>{{ isManagerNode(data) ? 'Manager' : 'Worker' }}</span>
              </span>
            </template>
          </Column>
          <Column header="并行任务" style="width: 120px" alignHeader="center" align="center">
            <template #body="{ data }">
              <div v-if="isManagerNode(data)" class="text-gray-300 font-medium">-</div>
              <div v-else class="flex items-center justify-center gap-1.5">
                <span :class="data.running_jobs > 0 ? 'text-blue-500 font-bold' : 'text-gray-600 font-medium'">
                  {{ data.running_jobs }}
                </span>
                <span class="text-gray-300 text-xs">/</span>
                <span class="text-gray-400 text-xs">
                  {{ data.max_concurrent > 0 ? data.max_concurrent : '∞' }}
                </span>
              </div>
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
              <div v-if="isRemovableNode(data)" class="action-buttons">
                <Button
                  v-tooltip.top="authStore.isAdmin ? '编辑标签' : '需管理员权限'"
                  icon="pi pi-pencil"
                  class="btn-edit"
                  :disabled="!authStore.isAdmin"
                  @click="handleEdit(data)"
                />
                <Button
                  v-tooltip.top="authStore.isAdmin ? '移除节点' : '需管理员权限'"
                  icon="pi pi-trash"
                  class="btn-delete"
                  :disabled="!authStore.isAdmin"
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
          <Chips v-model="editTags" separator=" " placeholder="输入标签并回车" class="w-full" autofocus />
        </div>
        <div class="flex flex-col gap-1">
          <label class="font-medium text-sm">最大并发任务数</label>
          <div class="flex items-center gap-3">
            <InputNumber v-model="editMaxConcurrent" :min="0" :max="1000" showButtons class="flex-1" placeholder="0 表示不限制" />
            <span class="text-xs text-gray-400 w-24">{{ editMaxConcurrent === 0 ? '不限制' : '项任务' }}</span>
          </div>
        </div>
        <p class="text-gray-400 text-xs">设置该节点允许同时运行的最大任务数量，0 为不限制。</p>
      </div>
      <template #footer>
        <Button severity="secondary" @click="editDialogVisible = false" label="取消" />
        <Button severity="info" :loading="editLoading" @click="saveEdit" label="保存" />
      </template>
    </Dialog>

    <Dialog v-model:visible="strategyDialogVisible" header="负载均衡策略管理" :style="{ width: '900px' }" :maximizable="true">
      <div class="flex flex-col gap-4">
        <div class="flex items-center justify-between">
          <span class="text-gray-400 text-sm">配置节点选择策略，基于资源指标加权评分选最优节点</span>
          <Button :label="authStore.isAdmin ? '新建策略' : '需管理员权限'" icon="pi pi-plus" size="small" :disabled="!authStore.isAdmin" @click="openStrategyForm()" />
        </div>

        <DataTable :value="strategies" stripedRows dataKey="id" class="strategy-table" :rowHover="true">
          <Column field="name" header="策略名称" style="min-width: 140px">
            <template #body="{ data }">
              <span class="font-semibold text-sm">{{ data.name }}</span>
            </template>
          </Column>
          <Column header="指标数" style="width: 80px" alignHeader="center" align="center">
            <template #body="{ data }">
              <span class="text-sm font-mono">{{ getMetricCount(data) }}</span>
            </template>
          </Column>
          <Column field="description" header="描述" style="min-width: 160px">
            <template #body="{ data }">
              <span class="text-gray-400 text-sm">{{ data.description || '-' }}</span>
            </template>
          </Column>
          <Column header="操作" style="width: 120px" frozen alignFrozen="right">
            <template #body="{ data }">
              <div class="action-buttons">
                <Button v-tooltip.top="authStore.isAdmin ? '编辑' : '需管理员权限'" icon="pi pi-pencil" class="btn-edit" :disabled="!authStore.isAdmin" @click="openStrategyForm(data)" />
                <Button v-tooltip.top="authStore.isAdmin ? '删除' : '需管理员权限'" icon="pi pi-trash" class="btn-delete" :disabled="!authStore.isAdmin" @click="deleteStrategy(data)" />
              </div>
            </template>
          </Column>
        </DataTable>

        <div v-if="strategies.length === 0" class="text-center py-8 text-gray-400">
          <i class="pi pi-scales text-4xl mb-2 block"></i>
          <p>暂无策略，使用默认最小负载策略</p>
        </div>
      </div>
      <template #footer>
        <Button severity="secondary" @click="strategyDialogVisible = false" label="关闭" />
      </template>
    </Dialog>

    <Dialog v-model:visible="strategyFormVisible" :header="isEditingStrategy ? '编辑策略' : '新建策略'" :style="{ width: '780px' }" :maximizable="true">
      <div class="flex flex-col gap-4">
        <div class="grid grid-cols-2 gap-4">
          <div class="flex flex-col gap-1">
            <label class="font-medium text-sm">策略名称 <span class="text-red-400">*</span></label>
            <InputText v-model="currentStrategy.name" placeholder="如：综合资源均衡" />
          </div>
          <div class="flex flex-col gap-1">
            <label class="font-medium text-sm">优先方向</label>
            <Select v-model="currentStrategy.direction" :options="[
              { value: 'asc', label: '优先选最小值' },
              { value: 'desc', label: '优先选最大值' },
            ]" optionLabel="label" optionValue="value" placeholder="选择方向" class="w-full" />
          </div>
        </div>
        <div class="flex flex-col gap-1">
          <label class="font-medium text-sm">描述</label>
          <InputText v-model="currentStrategy.description" placeholder="策略说明（可选）" autofocus />
        </div>

        <!-- 指标列表 -->
        <div class="flex items-center justify-between">
          <label class="font-medium text-sm">评分指标</label>
          <div class="flex items-center gap-2">
            <Button label="参数参考" icon="pi pi-question-circle" text size="small" severity="secondary" @click="formulaParamVisible = !formulaParamVisible" />
            <Button label="添加指标" icon="pi pi-plus" text size="small" @click="addMetric" />
          </div>
        </div>

        <!-- 参数参考面板 -->
        <div v-if="formulaParamVisible" class="param-reference">
          <div class="param-header">
            <span class="font-semibold text-sm">可用公式参数</span>
            <Button icon="pi pi-times" text size="small" severity="secondary" @click="formulaParamVisible = false" />
          </div>
          <div class="grid grid-cols-2 gap-2 mt-2">
            <div v-for="p in formulaParams" :key="p.name" class="param-item" @click="insertParam(p)">
              <div class="flex items-center gap-2">
                <code class="param-name">{{ p.name }}</code>
                <span class="param-unit">({{ p.unit }})</span>
              </div>
              <span class="param-desc">{{ p.description }}</span>
            </div>
          </div>
          <div class="mt-3 p-2 bg-gray-50 rounded text-xs text-gray-500">
            <p class="font-semibold mb-1">公式示例：</p>
            <code>memory_usage_pct * 0.5 + cpu_usage_pct * 0.3</code><br>
            <code>max(memory_usage_pct, cpu_usage_pct)</code><br>
            <code>(threads_used / threads_total) * 100</code>
          </div>
        </div>

        <div v-for="(metric, idx) in currentStrategy.metrics" :key="metric.id" class="metric-row">
          <div class="metric-header">
            <span class="text-sm font-semibold text-gray-500">指标 {{ idx + 1 }}</span>
            <Button icon="pi pi-trash" text size="small" severity="danger" @click="removeMetric(idx)" />
          </div>
          <div class="grid grid-cols-4 gap-3">
            <div class="flex flex-col gap-1 col-span-1">
              <label class="text-xs text-gray-400">名称</label>
              <InputText v-model="metric.name" placeholder="如：内存压力" class="text-sm" />
            </div>
            <div class="flex flex-col gap-1 col-span-2 overflow-hidden">
              <label class="text-xs text-gray-400">公式</label>
              <div class="relative w-full">
                <InputText v-model="metric.formula" placeholder="memory_usage_pct * 0.5" class="text-sm font-mono w-full formula-input" @input="validateFormulaDebounced(metric)" />
                <span v-if="formulaValidation[metric.id]" class="formula-status">
                  <i v-if="formulaValidation[metric.id].valid" class="pi pi-check text-green-500"></i>
                  <i v-else class="pi pi-times text-red-500" v-tooltip.top="formulaValidation[metric.id].error"></i>
                </span>
              </div>
            </div>
            <div class="flex flex-col gap-1 overflow-hidden">
              <label class="text-xs text-gray-400">权重</label>
              <InputNumber v-model="metric.weight" :minFractionDigits="0" :maxFractionDigits="2" :min="0" class="text-sm w-full weight-input" />
            </div>
          </div>
          <div v-if="formulaValidation[metric.id] && !formulaValidation[metric.id].valid" class="text-red-400 text-xs mt-1 ml-1">
            {{ formulaValidation[metric.id].error }}
          </div>
        </div>

        <div v-if="currentStrategy.metrics.length === 0" class="text-center py-6 text-gray-300 border border-dashed rounded-lg">
          <i class="pi pi-plus-circle text-2xl mb-1 block"></i>
          <p class="text-sm">点击"添加指标"配置评分公式</p>
        </div>
      </div>
      <template #footer>
        <Button severity="secondary" @click="strategyFormVisible = false" label="取消" />
        <Button :loading="strategyLoading" @click="saveStrategy" :label="isEditingStrategy ? '保存' : '创建'" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.workers { }

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

.stat-card-clickable {
  cursor: pointer;
}

.stat-card-clickable:hover {
  border-color: var(--color-primary, #8b5cf6);
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
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 11px;
  color: var(--p-surface-400);
}

.pid-text, .uptime-text {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
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
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 11px;
  color: var(--p-surface-400);
  text-align: right;
  font-weight: 500;
}

.usage-low :deep(.p-progressbar-value) { background: #10b981; }
.usage-medium :deep(.p-progressbar-value) { background: #f59e0b; }
.usage-high :deep(.p-progressbar-value) { background: #ef4444; }

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

.badge-manager {
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

.strategy-table :deep(.p-datatable-tbody > tr > td) {
  padding: 10px 14px;
}

:deep(.offline-row) {
  opacity: 0.6;
  filter: grayscale(0.5);
}

.param-reference {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 14px;
}

.param-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.param-item {
  padding: 6px 8px;
  border-radius: 6px;
  background: white;
  border: 1px solid #e2e8f0;
  cursor: pointer;
  transition: all 0.15s ease;
}

.param-item:hover {
  border-color: #8b5cf6;
  background: #f5f3ff;
}

.param-name {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  font-size: 11px;
  color: #6d28d9;
  font-weight: 600;
}

.param-unit {
  font-size: 11px;
  color: #a1a1aa;
}

.param-desc {
  font-size: 11px;
  color: #71717a;
}

.metric-row {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  padding: 12px;
}

.metric-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.formula-status {
  position: absolute;
  right: 10px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 12px;
}

.metric-row :deep(.p-inputtext.font-mono) {
  line-height: 1.6;
  padding: 6px 10px;
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
}

.formula-input:deep(.p-inputtext) {
  padding-right: 2.5rem;
}

.weight-input :deep(.p-inputnumber-input) {
  width: 100%;
}

@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
