<script setup lang="ts">
import { ref } from 'vue'
import { userApi } from '@/api'
import { showToast } from '@/utils/toast'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'

const emit = defineEmits<{
  (e: 'success'): void
}>()

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)

async function handleSubmit() {
  if (!oldPassword.value || !newPassword.value || !confirmPassword.value) {
    showToast({ severity: 'warn', summary: '请填写所有字段', life: 3000 })
    return
  }
  if (newPassword.value.length < 6) {
    showToast({ severity: 'warn', summary: '新密码长度不能少于6位', life: 3000 })
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    showToast({ severity: 'warn', summary: '两次输入的密码不一致', life: 3000 })
    return
  }

  loading.value = true
  try {
    await userApi.changePassword({
      old_password: oldPassword.value,
      new_password: newPassword.value,
    })
    showToast({ severity: 'success', summary: '密码修改成功', life: 3000 })
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    emit('success')
  } catch (error: any) {
    showToast({ severity: 'error', summary: '修改失败', detail: error.response?.data?.error || '请重试', life: 5000 })
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <form @submit.prevent="handleSubmit" class="space-y-4">
    <div>
      <label class="block text-sm font-medium text-gray-700 mb-1">旧密码</label>
      <InputText v-model="oldPassword" type="password" class="w-full" placeholder="请输入旧密码" />
    </div>
    <div>
      <label class="block text-sm font-medium text-gray-700 mb-1">新密码</label>
      <InputText v-model="newPassword" type="password" class="w-full" placeholder="请输入新密码（至少6位）" />
    </div>
    <div>
      <label class="block text-sm font-medium text-gray-700 mb-1">确认新密码</label>
      <InputText v-model="confirmPassword" type="password" class="w-full" placeholder="请再次输入新密码" />
    </div>
    <div class="flex justify-end gap-3 pt-2">
      <Button
        type="submit"
        label="确认修改"
        :loading="loading"
        :disabled="loading"
      />
    </div>
  </form>
</template>
