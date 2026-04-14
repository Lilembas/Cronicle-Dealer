<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { SwitchButton } from '@element-plus/icons-vue'
import {
  Home,
  Calendar,
  FileText,
  Monitor,
  Terminal
} from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const timeNow = ref(new Date().toLocaleTimeString())
let intervalId: number | null = null
let isMounted = false // 添加挂载状态标志

// Tab 配置 - 使用 lucide-vue-next 图标组件
const tabs = ref([
  { id: '/dashboard', label: '仪表盘', icon: Home },
  { id: '/jobs', label: '任务管理', icon: Calendar },
  { id: '/events', label: '执行记录', icon: FileText },
  { id: '/workers', label: '节点管理', icon: Monitor },
  { id: '/shell', label: 'Shell 执行', icon: Terminal },
])

const activeTab = ref('')

// 更新时间 - 添加安全检查
function updateTime() {
  if (isMounted) { // 只在组件仍然挂载时更新
    timeNow.value = new Date().toLocaleTimeString()
  }
}

// 退出登录
function handleLogout() {
  ElMessage.success('已退出登录')
  authStore.logout()
  router.push('/login')
}

// Tab 切换
function switchTab(tabId: string) {
  activeTab.value = tabId
  router.push(tabId)
}

onMounted(() => {
  isMounted = true // 设置挂载标志
  intervalId = setInterval(updateTime, 500) as unknown as number
  activeTab.value = route.path
})

onUnmounted(() => {
  isMounted = false // 清除挂载标志
  if (intervalId !== null) {
    clearInterval(intervalId)
    intervalId = null
  }
})

// 监听路由变化
router.afterEach((to) => {
  if (isMounted) { // 只在组件仍然挂载时更新
    activeTab.value = to.path
  }
})
</script>

<template>
  <div class="layout-container">
    <!-- Header with Logo and User -->
    <div class="head-home">
      <div class="container">
        <svg class="logo-img" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect x="3" y="3" width="7" height="7" rx="1" fill="#3b82f6" />
          <rect x="14" y="3" width="7" height="7" rx="1" fill="#8b5cf6" />
          <rect x="3" y="14" width="7" height="7" rx="1" fill="#10b981" />
          <rect x="14" y="14" width="7" height="7" rx="1" fill="#f59e0b" />
        </svg>
        <div class="h1-head-home">Cronicle-Next</div>
      </div>
      <div class="head-user">
        <el-dropdown trigger="click">
          <span class="user-dropdown-trigger">{{ authStore.user?.fullName || 'Admin' }}</span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item @click="handleLogout">
                <el-icon><SwitchButton /></el-icon>
                <span>退出登录</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <!-- Tabs Navigation -->
    <div class="head-tab">
      <ul class="tabs">
        <li
          v-for="tab in tabs"
          :key="tab.id"
          :class="{ active: activeTab === tab.id }"
          @click="switchTab(tab.id)"
        >
          <span class="tab-icon">
            <component :is="tab.icon" :size="16" />
          </span>
          <span class="tab-label">{{ tab.label }}</span>
        </li>
      </ul>
      <div class="time-display">{{ timeNow }}</div>
    </div>

    <!-- Tab Content Container -->
    <div class="tab_container">
      <div class="tab_content">
        <router-view v-slot="{ Component }">
          <transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </div>
    </div>
  </div>
</template>

<style scoped>
.layout-container {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background-color: #c9e7f5;
}

.user-dropdown-trigger {
  cursor: pointer;
  padding: 6px 12px;
  border: 1px solid #999;
  border-radius: 6px;
  background: #f5f5f5;
  transition: all 0.2s ease;
}

.user-dropdown-trigger:hover {
  background: #e8e8e8;
  border-color: #777;
}

.page-fade-enter-active,
.page-fade-leave-active {
  transition: all 0.3s ease;
}

.page-fade-enter-from {
  opacity: 0;
  transform: translateY(10px);
}

.page-fade-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}
</style>
