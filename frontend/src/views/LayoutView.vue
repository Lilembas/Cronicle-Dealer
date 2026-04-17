<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, provide } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { showToast } from '@/utils/toast'
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import Tabs from 'primevue/tabs'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'
import ScrollTop from 'primevue/scrolltop'
import Tooltip from 'primevue/tooltip'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const timeNow = ref(new Date().toLocaleTimeString())
let intervalId: number | null = null
let isMounted = false

// Global refresh trigger - pages can provide a refresh handler
const globalRefreshHandler = ref<(() => void) | null>(null)
provide('globalRefreshHandler', globalRefreshHandler)

// Register tooltip directive
const vTooltip = Tooltip

const tabs = [
  { id: '/dashboard', label: '仪表盘', icon: 'pi pi-home' },
  { id: '/jobs', label: '任务管理', icon: 'pi pi-calendar' },
  { id: '/events', label: '执行记录', icon: 'pi pi-list' },
  { id: '/workers', label: '节点管理', icon: 'pi pi-server' },
  { id: '/shell', label: 'Shell 执行', icon: 'pi pi-terminal' },
]

const activeTab = computed(() => route.path)
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

function handleGlobalRefresh() {
  if (globalRefreshHandler.value) {
    globalRefreshHandler.value()
    showToast({ severity: 'info', summary: '已刷新', life: 1500 })
  }
}

onMounted(() => {
  isMounted = true
  intervalId = setInterval(updateTime, 500) as unknown as number
})

onUnmounted(() => {
  isMounted = false
  if (intervalId !== null) {
    clearInterval(intervalId)
    intervalId = null
  }
})
</script>

<template>
  <div class="layout-container">
    <!-- Header with Logo and User -->
    <div class="head-home">
      <div class="container">
        <svg class="logo-img" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect x="3" y="3" width="7" height="7" rx="1.5" fill="#3b82f6" />
          <rect x="14" y="3" width="7" height="7" rx="1.5" fill="#8b5cf6" />
          <rect x="3" y="14" width="7" height="7" rx="1.5" fill="#10b981" />
          <rect x="14" y="14" width="7" height="7" rx="1.5" fill="#f59e0b" />
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
          <i class="pi pi-chevron-down ml-2" style="font-size: 12px" />
        </Button>
        <Menu ref="menu" :model="userMenuItems" :popup="true" />
      </div>
    </div>

    <!-- Tabs Navigation -->
    <div class="head-tab">
      <Tabs :value="activeTab">
        <TabList>
          <Tab v-for="tab in tabs" :key="tab.id" :value="tab.id">
            <router-link v-slot="{ href, navigate }" :to="tab.id" custom>
              <a :href="href" @click="navigate" class="tab-link">
                <i :class="tab.icon" />
                <span>{{ tab.label }}</span>
              </a>
            </router-link>
          </Tab>
        </TabList>
      </Tabs>
      <div class="time-display">
          <i class="pi pi-clock mr-2" style="font-size: 12px; opacity: 0.7" />
          {{ timeNow }}
        </div>
        <Button
          text
          severity="secondary"
          icon="pi pi-refresh"
          class="refresh-btn"
          @click="handleGlobalRefresh"
          v-tooltip.left="'刷新当前页面'"
          aria-label="刷新当前页面"
          style="padding: 6px; border-radius: 6px"
        />
    </div>

    <!-- Tab Content Container -->
    <div class="main-content">
      <div class="content-inner">
        <router-view v-slot="{ Component }">
          <transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </div>
    </div>

    <ScrollTop />
  </div>
</template>

<style scoped>
.layout-container {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background-color: var(--color-bg);
}

.tab-link {
  display: flex;
  align-items: center;
  gap: 8px;
  color: inherit;
  text-decoration: none;
}

.tab-link i {
  font-size: 14px;
}

.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.2s ease;
}

.page-fade-enter-from {
  opacity: 0;
}

.page-fade-leave-to {
  opacity: 0;
}
</style>
