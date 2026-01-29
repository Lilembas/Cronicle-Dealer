<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Fold,
  Expand,
  HomeFilled,
  Clock,
  Document,
  Monitor,
  SwitchButton,
  User,
} from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const isCollapse = ref(false)
const activeMenu = ref('/dashboard')

// 监听路由变化更新激活菜单
router.afterEach((to) => {
  activeMenu.value = to.path
})

// 退出登录
const handleLogout = () => {
  ElMessage.success('已退出登录')
  authStore.logout()
  router.push('/login')
}

// 切换侧边栏折叠
const toggleCollapse = () => {
  isCollapse.value = !isCollapse.value
}
</script>

<template>
  <div class="layout-container">
    <!-- 侧边栏 -->
    <aside :class="['sidebar', isCollapse ? 'collapsed' : '']">
      <!-- Logo 区域 -->
      <div class="logo-section">
        <div class="logo-icon-wrapper">
          <svg class="logo-svg" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="3" y="3" width="7" height="7" rx="1" fill="url(#gradient1)" />
            <rect x="14" y="3" width="7" height="7" rx="1" fill="url(#gradient2)" />
            <rect x="3" y="14" width="7" height="7" rx="1" fill="url(#gradient3)" />
            <rect x="14" y="14" width="7" height="7" rx="1" fill="url(#gradient4)" />
            <defs>
              <linearGradient id="gradient1" x1="3" y1="3" x2="10" y2="10">
                <stop offset="0%" stop-color="#3b82f6" />
                <stop offset="100%" stop-color="#2563eb" />
              </linearGradient>
              <linearGradient id="gradient2" x1="14" y1="3" x2="21" y2="10">
                <stop offset="0%" stop-color="#8b5cf6" />
                <stop offset="100%" stop-color="#7c3aed" />
              </linearGradient>
              <linearGradient id="gradient3" x1="3" y1="14" x2="10" y2="21">
                <stop offset="0%" stop-color="#10b981" />
                <stop offset="100%" stop-color="#059669" />
              </linearGradient>
              <linearGradient id="gradient4" x1="14" y1="14" x2="21" y2="21">
                <stop offset="0%" stop-color="#f59e0b" />
                <stop offset="100%" stop-color="#d97706" />
              </linearGradient>
            </defs>
          </svg>
        </div>
        <transition name="logo-text">
          <span v-if="!isCollapse" class="logo-text">Cronicle</span>
        </transition>
      </div>

      <!-- 导航菜单 -->
      <nav class="nav-menu">
        <router-link
          to="/dashboard"
          class="nav-item"
          :class="{ active: activeMenu === '/dashboard' }"
        >
          <el-icon class="nav-icon"><HomeFilled /></el-icon>
          <transition name="menu-text">
            <span v-if="!isCollapse" class="nav-text">仪表盘</span>
          </transition>
        </router-link>

        <router-link
          to="/jobs"
          class="nav-item"
          :class="{ active: activeMenu === '/jobs' }"
        >
          <el-icon class="nav-icon"><Clock /></el-icon>
          <transition name="menu-text">
            <span v-if="!isCollapse" class="nav-text">任务管理</span>
          </transition>
        </router-link>

        <router-link
          to="/events"
          class="nav-item"
          :class="{ active: activeMenu === '/events' }"
        >
          <el-icon class="nav-icon"><Document /></el-icon>
          <transition name="menu-text">
            <span v-if="!isCollapse" class="nav-text">执行记录</span>
          </transition>
        </router-link>

        <router-link
          to="/nodes"
          class="nav-item"
          :class="{ active: activeMenu === '/nodes' }"
        >
          <el-icon class="nav-icon"><Monitor /></el-icon>
          <transition name="menu-text">
            <span v-if="!isCollapse" class="nav-text">节点管理</span>
          </transition>
        </router-link>

        <router-link
          to="/shell"
          class="nav-item"
          :class="{ active: activeMenu === '/shell' }"
        >
          <el-icon class="nav-icon"><Monitor /></el-icon>
          <transition name="menu-text">
            <span v-if="!isCollapse" class="nav-text">Shell 执行</span>
          </transition>
        </router-link>
      </nav>
    </aside>

    <!-- 主内容区域 -->
    <main class="main-container">
      <!-- 顶部导航栏 -->
      <header class="header">
        <button class="collapse-btn" @click="toggleCollapse" :title="isCollapse ? '展开菜单' : '收起菜单'">
          <el-icon :size="20">
            <Fold v-if="!isCollapse" />
            <Expand v-else />
          </el-icon>
        </button>

        <div class="header-right">
          <!-- 用户下拉菜单 -->
          <el-dropdown trigger="click">
            <button class="user-btn">
              <div class="user-avatar">
                <el-icon><User /></el-icon>
              </div>
              <span class="user-name">{{ authStore.user?.fullName || 'Admin' }}</span>
            </button>
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
      </header>

      <!-- 页面内容 -->
      <div class="content-area">
        <router-view v-slot="{ Component }">
          <transition name="page-fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </div>
    </main>
  </div>
</template>

<style scoped>
/* 布局容器 */
.layout-container {
  display: flex;
  height: 100vh;
  width: 100%;
  overflow: hidden;
}

/* 侧边栏 */
.sidebar {
  width: 260px;
  background: linear-gradient(180deg, #1e293b 0%, #0f172a 100%);
  border-right: 1px solid #334155;
  display: flex;
  flex-direction: column;
  transition: width 0.3s ease;
  flex-shrink: 0;
  z-index: 100;
}

.sidebar.collapsed {
  width: 80px;
}

/* Logo 区域 */
.logo-section {
  height: 64px;
  display: flex;
  align-items: center;
  padding: 0 20px;
  gap: 12px;
  border-bottom: 1px solid #334155;
  transition: padding 0.3s ease;
}

.sidebar.collapsed .logo-section {
  padding: 0;
  justify-content: center;
}

.logo-icon-wrapper {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.logo-svg {
  width: 32px;
  height: 32px;
}

.logo-text {
  font-size: 20px;
  font-weight: 700;
  color: #f8fafc;
  letter-spacing: -0.5px;
  white-space: nowrap;
}

/* 导航菜单 */
.nav-menu {
  flex: 1;
  padding: 16px 12px;
  overflow-y: auto;
  overflow-x: hidden;
}

.sidebar.collapsed .nav-menu {
  padding: 16px 8px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  margin-bottom: 4px;
  border-radius: 10px;
  color: #94a3b8;
  text-decoration: none;
  transition: all 0.2s ease;
  cursor: pointer;
  position: relative;
  overflow: hidden;
}

.nav-item:hover {
  background: rgba(59, 130, 246, 0.1);
  color: #f8fafc;
}

.nav-item.active {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  color: white;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.3);
}

.nav-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 24px;
  background: white;
  border-radius: 0 3px 3px 0;
}

.nav-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.nav-text {
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
}

/* 主内容区域 */
.main-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #f8fafc;
}

/* 顶部导航栏 */
.header {
  height: 64px;
  background: white;
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  flex-shrink: 0;
}

.collapse-btn {
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  color: #64748b;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.collapse-btn:hover {
  background: #f1f5f9;
  color: #3b82f6;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-btn {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px 6px 6px;
  border: 1px solid #e2e8f0;
  background: white;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.user-btn:hover {
  background: #f8fafc;
  border-color: #cbd5e1;
}

.user-avatar {
  width: 32px;
  height: 32px;
  background: linear-gradient(135deg, #3b82f6 0%, #8b5cf6 100%);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
}

.user-name {
  font-size: 14px;
  font-weight: 500;
  color: #1e293b;
}

/* 内容区域 */
.content-area {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

/* 动画效果 */
.logo-text-enter-active,
.logo-text-leave-active {
  transition: all 0.3s ease;
}

.logo-text-enter-from,
.logo-text-leave-to {
  opacity: 0;
  transform: translateX(-10px);
}

.menu-text-enter-active,
.menu-text-leave-active {
  transition: all 0.3s ease;
}

.menu-text-enter-from,
.menu-text-leave-to {
  opacity: 0;
  width: 0;
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

/* 响应式设计 */
@media (max-width: 768px) {
  .sidebar {
    position: absolute;
    height: 100%;
    box-shadow: 4px 0 12px rgba(0, 0, 0, 0.1);
  }

  .sidebar.collapsed {
    transform: translateX(-100%);
  }

  .content-area {
    padding: 16px;
  }

  .header {
    padding: 0 16px;
  }
}
</style>
