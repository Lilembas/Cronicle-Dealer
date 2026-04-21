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
            { path: '', redirect: '/dashboard' },
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
                path: 'jobs/new',
                name: 'JobCreate',
                component: () => import('@/views/JobEditView.vue'),
                meta: { title: '新建任务' }
            },
            {
                path: 'jobs/:id/detail',
                name: 'JobDetail',
                component: () => import('@/views/JobDetailView.vue'),
                meta: { title: '任务详情' }
            },
            {
                path: 'jobs/:id/history',
                name: 'JobHistory',
                component: () => import('@/views/JobHistoryView.vue'),
                meta: { title: '执行历史' }
            },
            {
                path: 'jobs/:id',
                name: 'JobEdit',
                component: () => import('@/views/JobEditView.vue'),
                meta: { title: '编辑任务' }
            },
            {
                path: 'events',
                name: 'Events',
                component: () => import('@/views/EventsView.vue'),
                meta: { title: '执行记录' }
            },
            {
                path: 'workers',
                name: 'Workers',
                component: () => import('@/views/WorkersView.vue'),
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
            },
            {
                path: 'admin',
                name: 'Admin',
                component: () => import('@/views/admin/AdminView.vue'),
                meta: { title: '管理员', requiresAdmin: true },
                redirect: '/admin/users',
                children: [
                    {
                        path: 'users',
                        name: 'AdminUsers',
                        component: () => import('@/views/admin/UsersView.vue'),
                        meta: { title: '用户管理' }
                    },
                    {
                        path: 'logs',
                        name: 'AdminLogs',
                        component: () => import('@/views/admin/LogsView.vue'),
                        meta: { title: '管理日志' }
                    },
                    {
                        path: 'categories',
                        name: 'AdminCategories',
                        component: () => import('@/views/admin/CategoriesView.vue'),
                        meta: { title: '分组管理' }
                    },
                ]
            }
        ]
    }
]

const router = createRouter({
    history: createWebHistory('/'),
    routes
})

function isAdmin(): boolean {
    try {
        const user = JSON.parse(localStorage.getItem('auth_user') || 'null')
        return user?.role === 'admin'
    } catch {
        return false
    }
}

router.beforeEach((to, _from, next) => {
    const token = localStorage.getItem('auth_token')

    if (!token && to.meta.requiresAuth !== false) {
        return next({ name: 'Login' })
    }

    if (token && to.name === 'Login') {
        return next({ name: 'Dashboard' })
    }

    if (to.meta.requiresAdmin && !isAdmin()) {
        return next({ name: 'Dashboard' })
    }

    next()
})

export default router
