<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { statsApi, nodesApi, type Stats, type Node } from '@/api'
import { RefreshRight, CircleCheck, CircleClose, Loading } from '@element-plus/icons-vue'


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
</script>

<template>
  <div class="dashboard">
    <div class="page-header">
      <h2 class="page-title">仪表盘</h2>
      <el-button :icon="RefreshRight" @click="refetchStats">刷新</el-button>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-grid" v-loading="statsLoading">
      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon bg-blue">
            <el-icon><Clock /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-label">总任务数</div>
            <div class="stat-value">{{ stats?.total_jobs || 0 }}</div>
            <div class="stat-sub">已启用: {{ stats?.enabled_jobs || 0 }}</div>
          </div>
        </div>
      </el-card>

      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon bg-green">
            <el-icon><CircleCheck /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-label">成功执行</div>
            <div class="stat-value text-green">{{ stats?.success_events || 0 }}</div>
            <div class="stat-sub">总执行: {{ stats?.total_events || 0 }}</div>
          </div>
        </div>
      </el-card>

      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon bg-red">
            <el-icon><CircleClose /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-label">失败执行</div>
            <div class="stat-value text-red">{{ stats?.failed_events || 0 }}</div>
            <div class="stat-sub">运行中: {{ stats?.running_events || 0 }}</div>
          </div>
        </div>
      </el-card>

      <el-card class="stat-card">
        <div class="stat-content">
          <div class="stat-icon bg-purple">
            <el-icon><Monitor /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-label">在线节点</div>
            <div class="stat-value text-purple">{{ stats?.online_nodes || 0 }}</div>
            <div class="stat-sub">离线: {{ stats?.offline_nodes || 0 }}</div>
          </div>
        </div>
      </el-card>
    </div>

    <!-- 节点状态列表 -->
    <el-card class="mt-6">
      <template #header>
        <div class="card-header">
          <span class="font-semibold">节点状态</span>
        </div>
      </template>

      <div v-loading="nodesLoading">
        <el-table :data="nodes || []" stripe>
          <el-table-column prop="hostname" label="主机名" min-width="150" />
          <el-table-column prop="ip" label="IP 地址" width="150" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.status === 'online' ? 'success' : 'danger'">
                {{ row.status === 'online' ? '在线' : '离线' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="CPU" width="120">
            <template #default="{ row }">
              <el-progress :percentage="Math.round(row.cpu_usage)" :color="getProgressColor(row.cpu_usage)" />
            </template>
          </el-table-column>
          <el-table-column label="内存" width="120">
            <template #default="{ row }">
              <el-progress :percentage="Math.round(row.memory_percent)" :color="getProgressColor(row.memory_percent)" />
            </template>
          </el-table-column>
          <el-table-column label="磁盘" width="120">
            <template #default="{ row }">
              <el-progress :percentage="Math.round(row.disk_percent)" :color="getProgressColor(row.disk_percent)" />
            </template>
          </el-table-column>
          <el-table-column prop="running_jobs" label="运行任务" width="100" />
          <el-table-column prop="version" label="版本" width="100" />
        </el-table>

        <el-empty v-if="!nodesLoading && (!nodes || nodes.length === 0)" description="暂无节点数据" />
      </div>
    </el-card>
  </div>
</template>

<script lang="ts">
import { Clock, Monitor } from '@element-plus/icons-vue'

export default {
  components: { Clock, Monitor },
  methods: {
    getProgressColor(percentage: number) {
      if (percentage < 60) return '#67c23a'
      if (percentage < 80) return '#e6a23c'
      return '#f56c6c'
    }
  }
}
</script>

<style scoped>
.dashboard {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-title {
  font-size: 24px;
  font-weight: 600;
  color: #333;
  margin: 0;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

.stat-card {
  border-radius: 8px;
  transition: transform 0.3s, box-shadow 0.3s;
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.stat-content {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 60px;
  height: 60px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  color: white;
}

.stat-icon.bg-blue { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); }
.stat-icon.bg-green { background: linear-gradient(135deg, #84fab0 0%, #8fd3f4 100%); }
.stat-icon.bg-red { background: linear-gradient(135deg, #fa709a 0%, #fee140 100%); }
.stat-icon.bg-purple { background: linear-gradient(135deg, #a8edea 0%, #fed6e3 100%); }

.stat-info {
  flex: 1;
}

.stat-label {
  font-size: 14px;
  color: #666;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 32px;
  font-weight: 700;
  color: #333;
  line-height: 1;
  margin-bottom: 4px;
}

.stat-value.text-green { color: #67c23a; }
.stat-value.text-red { color: #f56c6c; }
.stat-value.text-purple { color: #9c27b0; }

.stat-sub {
  font-size: 12px;
  color: #999;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.mt-6 {
  margin-top: 24px;
}

.font-semibold {
  font-weight: 600;
}
</style>
