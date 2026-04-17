<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast } from '@/utils/toast'
import Button from 'primevue/button'
import Menu from 'primevue/menu'
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
let isMounted = false

// Tab 配置 - 使用 lucide-vue-next 图标组件
const tabs = ref([
  { id: '/dashboard', label: '仪表盘', icon: Home },
  { id: '/jobs', label: '任务管理', icon: Calendar },
  { id: '/events', label: '执行记录', icon: FileText },
  { id: '/workers', label: '节点管理', icon: Monitor },
  { id: '/shell', label: 'Shell 执行', icon: Terminal },
])

const activeTab = ref('')
const menu = ref()

const userMenuItems = ref([
  {
    label: '退出登录',
    icon: 'pi pi-sign-out',
    command: handleLogout
  }
])

function updateTime() {
  if (isMounted) {
    timeNow.value = new Date().toLocaleTimeString()
  }
}

function handleLogout() {
  showToast({ severity: 'success', summary: '已退出登录', life: 3000 })
  authStore.logout()
  router.push('/login')
}

function switchTab(tabId: string) {
  activeTab.value = tabId
  router.push(tabId)
}

onMounted(() => {
  isMounted = true
  intervalId = setInterval(updateTime, 500) as unknown as number
  activeTab.value = route.path
})

onUnmounted(() => {
  isMounted = false
  if (intervalId !== null) {
    clearInterval(intervalId)
    intervalId = null
  }
})

router.afterEach((to) => {
  if (isMounted) {
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
        <Button
          text
          severity="secondary"
          class="user-dropdown-trigger"
          @click="menu.toggle($event)"
        >
          {{ authStore.user?.fullName || 'Admin' }}
          <i class="pi pi-chevron-down ml-2" />
        </Button>
        <Menu ref="menu" :model="userMenuItems" :popup="true" />
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
