<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { showToast } from '@/utils/toast'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import Button from 'primevue/button'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api/auth'

const router = useRouter()
const authStore = useAuthStore()


const loginForm = ref({
  username: 'admin',
  password: 'admin123',
})

const loading = ref(false)

const handleLogin = async () => {
  if (!loginForm.value.username || !loginForm.value.password) {
    showToast({ severity: 'warn', summary: '提示', detail: '请输入用户名和密码', life: 3000 })
    return
  }

  loading.value = true

  try {
    const resp = await authApi.login({
      username: loginForm.value.username,
      password: loginForm.value.password,
    })

    authStore.setToken(resp.token)
    authStore.setUser({
      id: resp.user.id,
      username: resp.user.username,
      role: resp.user.role,
      fullName: resp.user.full_name,
    })

    showToast({ severity: 'success', summary: '登录成功', life: 3000 })

    await nextTick()
    await router.push('/dashboard')
  } catch {
    showToast({ severity: 'error', summary: '登录失败', detail: '请检查用户名和密码', life: 5000 })
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <!-- Logo 和标题 -->
      <div class="text-center mb-8">
        <div class="login-logo">
          <svg class="logo-img" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect x="3" y="3" width="7" height="7" rx="1.5" fill="#3b82f6" />
            <rect x="14" y="3" width="7" height="7" rx="1.5" fill="#8b5cf6" />
            <rect x="3" y="14" width="7" height="7" rx="1.5" fill="#10b981" />
            <rect x="14" y="14" width="7" height="7" rx="1.5" fill="#f59e0b" />
          </svg>
        </div>
        <h1 class="text-2xl font-bold text-gray-800 mb-1">Cronicle-Next</h1>
        <p class="text-gray-400 text-sm">分布式任务调度平台</p>
      </div>

      <!-- 登录表单 -->
      <form @submit.prevent="handleLogin">
        <div class="mb-4">
          <span class="p-input-icon-left w-full">
            <i class="pi pi-user" />
            <InputText v-model="loginForm.username" placeholder="用户名" class="w-full" />
          </span>
        </div>

        <div class="mb-4">
          <span class="p-input-icon-left w-full">
            <i class="pi pi-lock" />
            <Password
              v-model="loginForm.password"
              placeholder="密码"
              :feedback="false"
              :toggleMask="true"
              class="w-full"
              inputClass="w-full"
              @keyup.enter="handleLogin"
            />
          </span>
        </div>

        <div class="mb-4">
          <Button
            type="submit"
            severity="info"
            :loading="loading"
            class="w-full login-btn"
            label="登录"
          />
        </div>
      </form>

      <!-- 提示信息 -->
      <div class="text-center text-sm text-gray-500 mt-6">
        <p>默认账号：admin / admin123</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #0f172a 100%);
  padding: 20px;
}

.login-card {
  background: var(--color-surface);
  border-radius: 16px;
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.2);
  padding: 40px 32px;
  width: 100%;
  max-width: 400px;
}

.login-logo {
  margin-bottom: 16px;
}

.login-logo .logo-img {
  height: 40px;
  width: auto;
}

.login-btn {
  background: var(--color-brand);
  border: none;
  font-weight: 600;
}

.login-btn:hover {
  background: #2563eb !important;
}
</style>
