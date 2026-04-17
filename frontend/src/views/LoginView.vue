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
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-500 to-purple-600">
    <div class="bg-white rounded-2xl shadow-2xl p-8 w-full max-w-md">
      <!-- Logo 和标题 -->
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-gray-800 mb-2">Cronicle-Next</h1>
        <p class="text-gray-500">分布式任务调度平台</p>
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
.login-btn {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
}
.login-btn:hover {
  background: linear-gradient(135deg, #764ba2 0%, #667eea 100%) !important;
}
</style>
