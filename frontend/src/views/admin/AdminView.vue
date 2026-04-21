<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import Tabs from 'primevue/tabs'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'

const route = useRoute()

const adminTabs = [
  { id: '/admin/users', label: '用户管理', icon: 'pi pi-users' },
  { id: '/admin/logs', label: '管理日志', icon: 'pi pi-file' },
  { id: '/admin/categories', label: '分组管理', icon: 'pi pi-tags' },
]

const activeTab = computed(() => route.path)
</script>

<template>
  <div class="admin-page">
    <div class="admin-subtabs mb-4">
      <Tabs :value="activeTab">
        <TabList>
          <Tab v-for="tab in adminTabs" :key="tab.id" :value="tab.id">
            <router-link v-slot="{ href, navigate }" :to="tab.id" custom>
              <a :href="href" @click="navigate" class="tab-link">
                <i :class="tab.icon" />
                <span>{{ tab.label }}</span>
              </a>
            </router-link>
          </Tab>
        </TabList>
      </Tabs>
    </div>
    <router-view v-slot="{ Component }">
      <transition name="page-fade" mode="out-in">
        <component :is="Component" />
      </transition>
    </router-view>
  </div>
</template>

<style scoped>
.tab-link {
  display: flex;
  align-items: center;
  gap: 8px;
  color: inherit;
  text-decoration: none;
  padding: 10px 16px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.tab-link i {
  font-size: 14px;
}

:deep(.p-tab) {
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1) !important;
  border-radius: 8px 8px 0 0 !important;
}

:deep(.p-tab:not(.p-tab-active):hover) {
  background: #f8fafc !important;
}

:deep(.p-tab-active) {
  background: #eff6ff !important;
}

:deep(.p-tab-active .tab-link) {
  font-weight: 600;
  color: #2563eb !important;
}

:deep(.p-tab-active)::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 15%;
  right: 15%;
  height: 3px;
  background: #3b82f6 !important;
  border-radius: 3px 3px 0 0;
}

.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.2s ease;
}

.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>
