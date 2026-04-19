<script setup lang="ts">
import { ref, computed, provide, onMounted, onUnmounted, defineComponent, h } from 'vue'
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
import { useSystemStore } from '@/stores/system'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const systemStore = useSystemStore()

onMounted(() => {
  systemStore.syncTime()
  systemStore.startClock()
})

onUnmounted(() => {
  systemStore.stopClock()
})

const globalRefreshHandler = ref<(() => void) | null>(null)
provide('globalRefreshHandler', globalRefreshHandler)

// Register tooltip directive
const vTooltip = Tooltip

const TimeDisplay = defineComponent({
  setup() {
    const timeNow = ref(new Date().toLocaleTimeString())
    let intervalId: number | null = null
    function updateTime() {
      timeNow.value = new Date().toLocaleTimeString()
    }
    onMounted(() => {
      intervalId = setInterval(updateTime, 500) as unknown as number
    })
    onUnmounted(() => {
      if (intervalId !== null) { clearInterval(intervalId); intervalId = null }
    })
    return () => h('span', { class: 'time-display' }, timeNow.value)
  }
})

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

function handleLogout() {
  showToast({ severity: 'success', summary: '已退出登录', life: 3000 })
  authStore.logout()
  router.push('/login')
}

function handleGlobalRefresh() {
  if (globalRefreshHandler.value) {
    globalRefreshHandler.value()
    showToast({ severity: 'info', summary: '已刷新', life: 1000 })
  }
}
</script>

<template>
  <div class="layout-container">
    <!-- Header with Logo and User -->
    <div class="head-home">
      <div class="container">
        <img src="@/assets/logo.svg" class="logo-img" alt="Cronicle-Next" />
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
          <Tab v-for="tab in tabs" :key="tab.id" :value="tab.id" :pt="{ 
            root: { style: { padding: 0 } }
          }">
            <router-link v-slot="{ href, navigate }" :to="tab.id" custom>
              <a :href="href" @click="navigate" class="tab-link">
                <i :class="tab.icon" />
                <span>{{ tab.label }}</span>
              </a>
            </router-link>
          </Tab>
        </TabList>
      </Tabs>
      <div class="header-trailing">
          <Button
            text
            severity="secondary"
            icon="pi pi-refresh"
            class="refresh-btn"
            @click="handleGlobalRefresh"
            v-tooltip.bottom="'刷新'"
            aria-label="刷新"
          />
          <TimeDisplay />
        </div>
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
  padding: 12px 16px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tab-link i {
  font-size: 14px;
}

/* Tab Hover & Active Styling */
:deep(.p-tab) {
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1) !important;
  border-radius: 8px 8px 0 0 !important;
}

:deep(.p-tab:not(.p-tab-active):hover) {
  background: #f8fafc !important; /* slate-50 */
  transform: translateY(-1px);
}

:deep(.p-tab:not(.p-tab-active):hover .tab-link) {
  color: #3b82f6 !important; /* blue-500 */
}

:deep(.p-tab-active) {
  background: #eff6ff !important; /* blue-50 */
}

:deep(.p-tab-active .tab-link) {
  font-weight: 600;
  color: #2563eb !important; /* blue-600 */
  transform: scale(1.02);
}

:deep(.p-tab-active)::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 15%;
  right: 15%;
  height: 3px;
  background: #3b82f6 !important; /* blue-500 */
  border-radius: 3px 3px 0 0;
  box-shadow: 0 -2px 10px rgba(59, 130, 246, 0.3);
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
