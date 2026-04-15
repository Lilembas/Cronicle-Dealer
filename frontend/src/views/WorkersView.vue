<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { nodesApi, type Node } from '@/api'
import { useWebSocketStore } from '@/stores/websocket'
import { Delete, Edit, View } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const loading = ref(false)
const nodes = ref<Node[]>([])
const selectedNodes = ref<string[]>([])
const wsStore = useWebSocketStore()

// 编辑节点
const editDialogVisible = ref(false)
const editNode = ref<Node | null>(null)
const editTags = ref<string[]>([])
const editLoading = ref(false)

// Master 节点标识（通过 tags 字段判断）
const isMasterNode = (node: Node) => {
  return node.tags === 'master' || node.tags.includes('master')
}

// 获取 Master 节点（如果没有明确的 Master，则使用最早注册的节点）
const getMasterNodeId = () => {
  const nodesWithId = nodes.value.map(node => ({
    ...node,
    isMaster: isMasterNode(node)
  }))

  // 优先返回明确标记为 Master 的节点
  const explicitMaster = nodesWithId.find(node => node.isMaster)
  if (explicitMaster) {
    return explicitMaster.id
  }

  // 如果没有明确的 Master，返回最早注册的节点
  if (nodesWithId.length > 0) {
    const sortedByRegistration = [...nodesWithId].sort((a, b) =>
      new Date(a.registered_at).getTime() - new Date(b.registered_at).getTime()
    )
    return sortedByRegistration[0].id
  }

  return null
}

// 格式化运行时间
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


// 过滤后的节点列表（在线节点优先，master 节点始终在第一位）
const filteredNodes = computed(() => {
  let result = [...nodes.value]

  // 排序：在线优先，然后是 Master 优先，最后按注册时间
  result.sort((a: Node, b: Node) => {
    // 首先按在线状态排序（在线优先）
    if (a.status === 'online' && b.status !== 'online') return -1
    if (a.status !== 'online' && b.status === 'online') return 1

    // 然后按 Master 角色排序（Master 优先）
    const aIsMaster = isMasterNode(a)
    const bIsMaster = isMasterNode(b)
    if (aIsMaster && !bIsMaster) return -1
    if (!aIsMaster && bIsMaster) return 1

    // 最后按注册时间排序（最早的在前）
    return new Date(a.registered_at).getTime() - new Date(b.registered_at).getTime()
  })

  return result
})

// 获取使用率样式类
const getUsageClass = (value: number) => {
  if (value < 60) return 'usage-low'
  if (value < 80) return 'usage-medium'
  return 'usage-high'
}

// 获取行样式（离线节点显示灰色）
const getRowClass = ({ row }: { row: any }) => {
  if (row.status === 'offline') {
    return 'offline-row'
  }
  return ''
}

// 加载节点列表
const loadNodes = async () => {
  try {
    loading.value = true
    const data = await nodesApi.list()
    nodes.value = (data as any) || []

    // 调试：打印节点数据
    console.log('加载的节点数据:', nodes.value)
    console.log('节点 PID 信息:', nodes.value.map(node => ({
      hostname: node.hostname,
      ip: node.ip,
      pid: node.pid,
      status: node.status
    })))
    console.log('Master 节点判断:', nodes.value.map(node => ({
      hostname: node.hostname,
      ip: node.ip,
      isMaster: isMasterNode(node)
    })))
  } catch (error) {
    ElMessage.error('加载节点失败')
    console.error('加载节点失败:', error)
  } finally {
    loading.value = false
  }
}

// 删除单个节点
const handleDelete = async (node: Node) => {
  // 检查是否是 master 节点
  if (isMasterNode(node) || node.id === getMasterNodeId()) {
    ElMessage.warning('不能删除 Master 节点')
    return
  }

  try {
    await ElMessageBox.confirm(
      `确定要删除节点 "${node.hostname}" (${node.ip}) 吗？`,
      '删除 Worker 节点',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )

    await nodesApi.delete(node.id)
    ElMessage.success('删除成功')
    await loadNodes()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.error || '删除失败')
    }
  }
}

// 打开编辑对话框
const handleEdit = (node: Node) => {
  editNode.value = node
  // 解析 tags 字段（可能是 JSON 数组字符串）
  try {
    editTags.value = node.tags ? JSON.parse(node.tags) : []
  } catch {
    editTags.value = node.tags ? [node.tags] : []
  }
  editDialogVisible.value = true
}

// 保存编辑
const saveEdit = async () => {
  if (!editNode.value) return

  try {
    editLoading.value = true
    await nodesApi.update(editNode.value.id, {
      tags: JSON.stringify(editTags.value)
    })
    ElMessage.success('更新成功')
    editDialogVisible.value = false
    await loadNodes()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '更新失败')
  } finally {
    editLoading.value = false
  }
}

// 处理节点状态更新
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
    <el-card class="table-card" shadow="never">
      <el-table
        :data="filteredNodes"
        stripe
        v-loading="loading"
        @selection-change="(selection: any[]) => selectedNodes = selection.map((n: any) => n.id)"
        :selectable="(row: Node) => !(isMasterNode(row) || row.id === getMasterNodeId())"
        :row-class-name="getRowClass"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="hostname" label="Hostname" min-width="150">
          <template #default="{ row }">
            <div class="hostname-cell">
              <span class="hostname-text">{{ row.hostname }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP Address" width="140" />
        <el-table-column label="PID" width="80" align="center">
          <template #default="{ row }">
            {{ row.pid || '-' }}
          </template>
        </el-table-column>
        <el-table-column label="Status" width="100" align="center">
          <template #default="{ row }">
            <el-tag
              v-if="isMasterNode(row)"
              size="small"
              type="warning"
              effect="dark"
            >
              Master
            </el-tag>
            <el-tag v-else size="small" type="primary" effect="dark">
              Worker
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="Active Jobs" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.running_jobs > 0" size="small" type="success">
              {{ row.running_jobs }}
            </el-tag>
            <span v-else class="text-none">(None)</span>
          </template>
        </el-table-column>
        <el-table-column label="Uptime" width="100">
          <template #default="{ row }">
            {{ formatUptime(row.registered_at) }}
          </template>
        </el-table-column>
        <el-table-column label="CPU" width="80" align="right">
          <template #default="{ row }">
            <span :class="getUsageClass(row.cpu_usage)">
              {{ (row.cpu_usage || 0).toFixed(1) }}%
            </span>
          </template>
        </el-table-column>
        <el-table-column label="Mem" width="80" align="right">
          <template #default="{ row }">
            <span :class="getUsageClass(row.memory_percent)">
              {{ (row.memory_percent || 0).toFixed(1) }}%
            </span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <div class="action-row">
              <el-tooltip content="编辑" placement="top">
                <el-button
                  v-if="!isMasterNode(row) && row.id !== getMasterNodeId()"
                  size="small"
                  :icon="Edit"
                  @click="handleEdit(row)"
                />
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button
                  v-if="!isMasterNode(row) && row.id !== getMasterNodeId()"
                  size="small"
                  type="danger"
                  :icon="Delete"
                  @click="handleDelete(row)"
                />
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && filteredNodes.length === 0" description="暂无节点数据" />
    </el-card>

    <!-- 编辑节点对话框 -->
    <el-dialog
      v-model="editDialogVisible"
      title="编辑节点标签"
      width="500px"
    >
      <el-form v-if="editNode" label-width="100px">
        <el-form-item label="节点">
          <el-input :model-value="editNode.hostname" disabled />
        </el-form-item>
        <el-form-item label="IP 地址">
          <el-input :model-value="editNode.ip" disabled />
        </el-form-item>
        <el-form-item label="节点标签">
          <el-select
            v-model="editTags"
            multiple
            filterable
            allow-create
            default-first-option
            placeholder="添加标签（按回车确认）"
            style="width: 100%"
          >
            <el-option
              v-for="tag in editTags"
              :key="tag"
              :label="tag"
              :value="tag"
            />
          </el-select>
        </el-form-item>
        <div style="color: #909399; font-size: 12px; margin-left: 100px">
          节点标签用于任务调度时按标签匹配 Worker 节点
        </div>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="editLoading" @click="saveEdit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.workers-page {
  padding: 24px;
  max-width: 1600px;
  margin: 0 auto;
}

/* 表格 */
.table-card {
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.hostname-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.hostname-text {
  font-weight: 500;
  color: #1e293b;
}

.text-none {
  color: #94a3b8;
  font-size: 12px;
}

/* 离线行样式 */
:deep(.offline-row) {
  background-color: #f5f5f5 !important;
  color: #999 !important;
}

:deep(.offline-row:hover) {
  background-color: #e8e8e8 !important;
}

:deep(.offline-row .hostname-text) {
  color: #999 !important;
}

:deep(.offline-row .el-tag) {
  opacity: 0.6;
}

/* CPU 使用率颜色 */
.usage-low {
  color: #10b981;
  font-weight: 500;
}

.usage-medium {
  color: #f59e0b;
  font-weight: 500;
}

.usage-high {
  color: #ef4444;
  font-weight: 500;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .workers-page {
    padding: 16px;
  }
}

/* 操作按钮行 */
.action-row {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: nowrap;
}

.action-row :deep(.el-button) {
  padding: 4px;
  margin: 0;
}
</style>
