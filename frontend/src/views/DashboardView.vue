<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { statsApi, nodesApi } from '@/api'
import { RefreshRight, CircleCheck, CircleClose, Monitor, Clock } from '@element-plus/icons-vue'

// 获取统计数据
const { data: stats, isLoading: statsLoading, refetch: refetchStats } = useQuery({
  queryKey: ['stats'],
  queryFn: statsApi.get,
  refetchInterval: 5000, // 每 5 秒刷新一次
})

// 获取节点列表
const { data: nodes, isLoading: nodesLoading } = useQuery({
  queryKey: ['nodes'],
  queryFn: () => nodesApi.list(),
  refetchInterval: 10000, // 每 10 秒刷新一次
})

// 进度条颜色
const getProgressColor = (percentage: number) => {
  if (percentage < 60) return '#10b981'
  if (percentage < 80) return '#f59e0b'
  return '#ef4444'
}
</script>

<template>
  <div class="dashboard">
    <!-- 页面标题 -->
    <div class="page-header">
      <div>
        <h1 class="page-title">仪表盘</h1>
        <p class="page-subtitle">实时监控任务调度和节点状态</p>
      </div>
      <el-button
        type="primary"
        :icon="RefreshRight"
        @click="refetchStats()"
        class="refresh-btn"
      >
        刷新
      </el-button>
    </div>

    <!-- 统计卡片网格 -->
    <div class="stats-grid" v-loading="statsLoading">
      <!-- 总任务数 -->
      <div class="stat-card">
        <div class="stat-card-inner">
          <div class="stat-icon stat-icon-blue">
            <el-icon :size="28"><Clock /></el-icon>
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
            <el-icon :size="28"><CircleCheck /></el-icon>
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
            <el-icon :size="28"><CircleClose /></el-icon>
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
            <el-icon :size="28"><Monitor /></el-icon>
          </div>
          <div class="stat-content">
            <div class="stat-label">在线节点</div>
            <div class="stat-value stat-value-nodes">{{ stats?.online_nodes || 0 }}</div>
            <div class="stat-sub">离线: {{ stats?.offline_nodes || 0 }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- 节点状态列表 -->
    <div class="nodes-section">
      <el-card class="nodes-card" shadow="never">
        <template #header>
          <div class="card-header">
            <h3 class="card-title">节点状态</h3>
            <el-tag size="small">{{ nodes?.length || 0 }} 个节点</el-tag>
          </div>
        </template>

        <div v-loading="nodesLoading">
          <el-table
            :data="nodes || []"
            stripe
            class="nodes-table"
            :empty-text="'暂无节点数据'"
          >
            <el-table-column prop="hostname" label="主机名" min-width="160" show-overflow-tooltip />
            <el-table-column prop="ip" label="IP 地址" width="140" />
            <el-table-column label="状态" width="100" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 'online' ? 'success' : 'info'" size="small">
                  {{ row.status === 'online' ? '在线' : '离线' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="CPU 使用率" width="160">
              <template #default="{ row }">
                <div class="progress-cell">
                  <el-progress
                    :percentage="Math.round(row.cpu_usage)"
                    :color="getProgressColor(row.cpu_usage)"
                    :stroke-width="6"
                  />
                </div>
              </template>
            </el-table-column>
            <el-table-column label="内存使用率" width="160">
              <template #default="{ row }">
                <div class="progress-cell">
                  <el-progress
                    :percentage="Math.round(row.memory_percent)"
                    :color="getProgressColor(row.memory_percent)"
                    :stroke-width="6"
                  />
                </div>
              </template>
            </el-table-column>
            <el-table-column label="磁盘使用率" width="160">
              <template #default="{ row }">
                <div class="progress-cell">
                  <el-progress
                    :percentage="Math.round(row.disk_percent)"
                    :color="getProgressColor(row.disk_percent)"
                    :stroke-width="6"
                  />
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="running_jobs" label="运行任务" width="100" align="center" />
            <el-table-column prop="version" label="版本" width="100" />
          </el-table>
        </div>
      </el-card>
    </div>
  </div>
</template>

<style scoped>
.dashboard {
  padding: 24px;
  max-width: 1600px;
  margin: 0 auto;
  background: #f8fafc;
  min-height: calc(100vh - 60px);
}

/* 页面标题 */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 32px;
  gap: 16px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
  margin: 0 0 4px 0;
  line-height: 1.2;
}

.page-subtitle {
  font-size: 14px;
  color: #64748b;
  margin: 0;
}

.refresh-btn {
  flex-shrink: 0;
}

/* 统计卡片网格 */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

.stat-card {
  background: white;
  border-radius: 16px;
  border: 1px solid #e2e8f0;
  overflow: hidden;
  transition: all 0.2s ease;
  cursor: pointer;
}

.stat-card:hover {
  border-color: #3b82f6;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.1);
}

.stat-card-inner {
  padding: 24px;
  display: flex;
  align-items: center;
  gap: 20px;
}

/* 统计图标 */
.stat-icon {
  width: 64px;
  height: 64px;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  flex-shrink: 0;
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
}

.stat-icon-blue {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
}

.stat-icon-green {
  background: linear-gradient(135deg, #10b981 0%, #059669 100%);
}

.stat-icon-red {
  background: linear-gradient(135deg, #ef4444 0%, #dc2626 100%);
}

.stat-icon-purple {
  background: linear-gradient(135deg, #8b5cf6 0%, #7c3aed 100%);
}

/* 统计内容 */
.stat-content {
  flex: 1;
  min-width: 0;
}

.stat-label {
  font-size: 13px;
  font-weight: 500;
  color: #64748b;
  margin-bottom: 8px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.stat-value {
  font-size: 36px;
  font-weight: 700;
  color: #1e293b;
  line-height: 1;
  margin-bottom: 6px;
}

.stat-value-success {
  color: #10b981;
}

.stat-value-failed {
  color: #ef4444;
}

.stat-value-nodes {
  color: #8b5cf6;
}

.stat-sub {
  font-size: 13px;
  color: #94a3b8;
}

/* 节点列表 */
.nodes-section {
  margin-top: 24px;
}

.nodes-card {
  border-radius: 16px;
  border: 1px solid #e2e8f0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: #1e293b;
  margin: 0;
}

.nodes-table {
  width: 100%;
}

.progress-cell {
  padding: 0 8px;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .dashboard {
    padding: 16px;
  }

  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .page-title {
    font-size: 24px;
  }

  .stats-grid {
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .stat-card-inner {
    padding: 20px;
  }

  .stat-value {
    font-size: 32px;
  }
}

@media (max-width: 640px) {
  .nodes-table :deep(.el-table__body-wrapper) {
    overflow-x: auto;
  }
}
</style>
