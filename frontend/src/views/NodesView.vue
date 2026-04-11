<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { nodesApi, type Node } from '@/api'
import { RefreshRight } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getWebSocketClient } from '@/utils/websocket'

const loading = ref(false)
const nodes = ref<Node[]>([])
const wsClient = getWebSocketClient()

const getProgressColor = (value: number) => {
  if (value < 60) return '#10b981'
  if (value < 80) return '#f59e0b'
  return '#ef4444'
}

const loadNodes = async () => {
  try {
    loading.value = true
    nodes.value = await nodesApi.list()
  } catch {
    ElMessage.error('加载节点失败')
  } finally {
    loading.value = false
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

  try {
    if (!wsClient['ws'] || wsClient['ws'].readyState !== WebSocket.OPEN) {
      await wsClient.connect()
    }
    wsClient.onMessage('node_status', handleNodeStatus)
    wsClient.joinRoom('global')
  } catch {
    // websocket 失败时使用手动刷新
  }
})

onUnmounted(() => {
  wsClient.offMessage('node_status', handleNodeStatus)
  wsClient.leaveRoom('global')
})
</script>

<template>
  <div class="nodes-page">
    <div class="page-header">
      <h2 class="page-title">节点管理</h2>
      <el-button :icon="RefreshRight" @click="loadNodes">刷新</el-button>
    </div>

    <el-card shadow="never">
      <el-table :data="nodes" stripe v-loading="loading">
        <el-table-column prop="hostname" label="主机名" min-width="180" />
        <el-table-column prop="ip" label="IP" width="150" />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'online' ? 'success' : 'info'">
              {{ row.status === 'online' ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="CPU" width="170">
          <template #default="{ row }">
            <el-progress
              :percentage="Math.round(row.cpu_usage || 0)"
              :color="getProgressColor(row.cpu_usage || 0)"
              :stroke-width="7"
            />
          </template>
        </el-table-column>
        <el-table-column label="内存" width="170">
          <template #default="{ row }">
            <el-progress
              :percentage="Math.round(row.memory_percent || 0)"
              :color="getProgressColor(row.memory_percent || 0)"
              :stroke-width="7"
            />
          </template>
        </el-table-column>
        <el-table-column label="磁盘" width="170">
          <template #default="{ row }">
            <el-progress
              :percentage="Math.round(row.disk_percent || 0)"
              :color="getProgressColor(row.disk_percent || 0)"
              :stroke-width="7"
            />
          </template>
        </el-table-column>
        <el-table-column prop="running_jobs" label="运行任务" width="100" align="center" />
        <el-table-column prop="version" label="版本" width="100" />
        <el-table-column label="最后心跳" min-width="180">
          <template #default="{ row }">
            {{ row.last_heartbeat ? new Date(row.last_heartbeat).toLocaleString('zh-CN') : '-' }}
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && nodes.length === 0" description="暂无节点数据" />
    </el-card>
  </div>
</template>

<style scoped>
.nodes-page {
  padding: 24px;
  max-width: 1500px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-title {
  margin: 0;
  font-size: 24px;
}
</style>
