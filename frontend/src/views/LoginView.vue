<script setup lang="ts">
import { ref, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'
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
    ElMessage.warning('请输入用户名和密码')
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

    ElMessage.success('登录成功')
    
    // 使用 nextTick 确保状态更新后再跳转
    await nextTick()
    await router.push('/dashboard')
  } catch {
    ElMessage.error('登录失败，请检查用户名和密码')
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
      <el-form :model="loginForm" @submit.prevent="handleLogin">
        <el-form-item>
          <el-input
            v-model="loginForm.username"
            :prefix-icon="User"
            placeholder="用户名"
            size="large"
          />
        </el-form-item>

        <el-form-item>
          <el-input
            v-model="loginForm.password"
            :prefix-icon="Lock"
            type="password"
            placeholder="密码"
            size="large"
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            class="w-full"
            @click="handleLogin"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <!-- 提示信息 -->
      <div class="text-center text-sm text-gray-500 mt-6">
        <p>默认账号：admin / admin123</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.el-button--primary {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border: none;
}

.el-button--primary:hover {
  background: linear-gradient(135deg, #764ba2 0%, #667eea 100%);
}
</style>
