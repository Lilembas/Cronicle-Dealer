import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
    {
        path: '/login',
        name: 'Login',
        component: () => import('@/views/LoginView.vue'),
        meta: { requiresAuth: false }
    },
    {
        path: '/',
        component: () => import('@/views/LayoutView.vue'),
        meta: { requiresAuth: true },
        children: [
            {
                path: '',
                redirect: '/dashboard'
            },
            {
                path: 'dashboard',
                name: 'Dashboard',
                component: () => import('@/views/DashboardView.vue'),
                meta: { title: '仪表盘' }
            },
            {
                path: 'jobs',
                name: 'Jobs',
                component: () => import('@/views/JobsView.vue'),
                meta: { title: '任务管理' }
            },
            {
                path: 'jobs/:id',
                name: 'JobDetail',
                component: () => import('@/views/JobDetailView.vue'),
                meta: { title: '任务详情' }
            },
            {
                path: 'events',
                name: 'Events',
                component: () => import('@/views/EventsView.vue'),
                meta: { title: '执行记录' }
            },
            {
                path: 'nodes',
                name: 'Nodes',
                component: () => import('@/views/NodesView.vue'),
                meta: { title: '节点管理' }
            },
            {
                path: 'logs/:id',
                name: 'Logs',
                component: () => import('@/views/LogsView.vue'),
                meta: { title: '日志查看' }
            },
            {
                path: 'shell',
                name: 'Shell',
                component: () => import('@/views/ShellView.vue'),
                meta: { title: 'Shell 执行' }
            }
        ]
    }
]

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    routes
})

// 路由守卫：检查认证状态
router.beforeEach((to, from, next) => {
    const token = localStorage.getItem('auth_token')

    if (to.meta.requiresAuth && !token) {
        next({ name: 'Login' })
    } else if (to.name === 'Login' && token) {
        next({ name: 'Dashboard' })
    } else {
        next()
    }
})

export default router
