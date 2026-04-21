<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { adminApi, type AdminUser } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import InputToggle from 'primevue/inputswitch'

const authStore = useAuthStore()
const loading = ref(false)
const users = ref<AdminUser[]>([])
const total = ref(0)

const editDialogVisible = ref(false)
const editUser = ref<Partial<AdminUser> & { password?: string } | null>(null)
const editLoading = ref(false)
const isEdit = computed(() => !!editUser.value?.id)

const roleOptions = [
  { label: '管理员', value: 'admin' },
  { label: '普通用户', value: 'user' },
  { label: '只读用户', value: 'viewer' },
]

const totalUsers = computed(() => users.value.length)
const activeUsers = computed(() => users.value.filter(u => u.active).length)
const adminCount = computed(() => users.value.filter(u => u.role === 'admin').length)
const activeAdminCount = computed(() => users.value.filter(u => u.role === 'admin' && u.active).length)
const disabledCount = computed(() => users.value.filter(u => !u.active).length)

async function loadUsers() {
  loading.value = true
  try {
    const res = await adminApi.listUsers({ page: 1, page_size: 100 }) as any
    users.value = res.data || []
    total.value = res.total || 0
  } catch {
    showToast({ severity: 'error', summary: '加载用户列表失败', life: 5000 })
  } finally {
    loading.value = false
  }
}

function openCreateDialog() {
  editUser.value = {
    username: '',
    password: '',
    email: '',
    role: 'user',
    full_name: '',
    active: true,
  }
  editDialogVisible.value = true
}

function openEditDialog(user: AdminUser) {
  editUser.value = { ...user, password: '' }
  editDialogVisible.value = true
}

async function handleSave() {
  if (!editUser.value) return
  if (!editUser.value.username) {
    showToast({ severity: 'warn', summary: '用户名不能为空', life: 3000 })
    return
  }
  if (!isEdit.value && !editUser.value.password) {
    showToast({ severity: 'warn', summary: '密码不能为空', life: 3000 })
    return
  }
  if (editUser.value.password && editUser.value.password.length < 6) {
    showToast({ severity: 'warn', summary: '密码长度不能少于6位', life: 3000 })
    return
  }

  editLoading.value = true
  try {
    if (isEdit.value) {
      const { username, email, role, full_name, active, password } = editUser.value
      
      // 禁止禁用或变更最后一个激活的管理员
      const isOriginalAdmin = users.value.find(u => u.id === editUser.value!.id)?.role === 'admin'
      const isChangedToNonAdmin = isOriginalAdmin && role !== 'admin'
      if (isOriginalAdmin && (!active || isChangedToNonAdmin) && activeAdminCount.value <= 1) {
        showToast({ severity: 'error', summary: '禁止操作', detail: '不能禁用或变更最后一个激活的管理员角色', life: 5000 })
        return
      }

      const data: any = { username, email, role, full_name, active }
      if (password) data.password = password
      await adminApi.updateUser(editUser.value.id!, data)
      showToast({ severity: 'success', summary: '用户更新成功', life: 3000 })
    } else {
      await adminApi.createUser({
        username: editUser.value.username!,
        password: editUser.value.password!,
        email: editUser.value.email,
        role: editUser.value.role || 'user',
        full_name: editUser.value.full_name,
      })
      showToast({ severity: 'success', summary: '用户创建成功', life: 3000 })
    }
    editDialogVisible.value = false
    await loadUsers()
  } catch (error: any) {
    showToast({ severity: 'error', summary: '操作失败', detail: error.response?.data?.error || '请重试', life: 5000 })
  } finally {
    editLoading.value = false
  }
}

async function handleDelete(user: AdminUser) {
  if (user.id === authStore.user?.id) {
    showToast({ severity: 'warn', summary: '禁止操作', detail: '不能删除自己', life: 3000 })
    return
  }
  
  if (user.role === 'admin' && adminCount.value <= 1) {
    showToast({ severity: 'warn', summary: '禁止操作', detail: '不能删除最后一个管理员', life: 3000 })
    return
  }

  showConfirm({
    message: `确定要删除用户 "${user.username}" 吗？`,
    header: '删除用户',
    accept: async () => {
      try {
        await adminApi.deleteUser(user.id)
        showToast({ severity: 'success', summary: '用户已删除', life: 3000 })
        await loadUsers()
      } catch (error: any) {
        showToast({ severity: 'error', summary: '删除失败', detail: error.response?.data?.error || '请重试', life: 5000 })
      }
    }
  })
}

function getRoleSeverity(role: string) {
  switch (role) {
    case 'admin': return 'danger'
    case 'user': return 'info'
    case 'viewer': return 'secondary'
    default: return 'info'
  }
}

function getRoleLabel(role: string) {
  switch (role) {
    case 'admin': return '管理员'
    case 'user': return '普通用户'
    case 'viewer': return '只读'
    default: return role
  }
}

function formatDate(dateStr?: string) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

onMounted(loadUsers)
</script>

<template>
  <div class="users-page">
    <div class="stats-grid mb-6">
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-blue-50 text-blue-500">
              <i class="pi pi-users text-xl"></i>
            </div>
            <div>
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">总用户</div>
              <div class="text-2xl font-bold">{{ totalUsers }}</div>
            </div>
          </div>
        </template>
      </Card>
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-green-50 text-green-500">
              <i class="pi pi-user-check text-xl"></i>
            </div>
            <div>
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">已启用</div>
              <div class="text-2xl font-bold">{{ activeUsers }}</div>
            </div>
          </div>
        </template>
      </Card>
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-amber-50 text-amber-500">
              <i class="pi pi-shield text-xl"></i>
            </div>
            <div>
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">管理员</div>
              <div class="text-2xl font-bold">{{ adminCount }}</div>
            </div>
          </div>
        </template>
      </Card>
      <Card class="stat-card">
        <template #content>
          <div class="flex items-center gap-4">
            <div class="stat-icon bg-red-50 text-red-500">
              <i class="pi pi-user-minus text-xl"></i>
            </div>
            <div>
              <div class="text-gray-400 text-xs font-semibold uppercase tracking-wider">已禁用</div>
              <div class="text-2xl font-bold">{{ disabledCount }}</div>
            </div>
          </div>
        </template>
      </Card>
    </div>

    <Card class="table-card">
      <template #content>
        <div class="flex justify-between items-center mb-4">
          <h3 class="text-lg font-semibold text-gray-800">用户列表</h3>
          <Button label="新建用户" icon="pi pi-plus" size="small" @click="openCreateDialog" />
        </div>
        <DataTable :value="users" stripedRows :loading="loading" dataKey="id">
          <Column field="username" header="用户名" style="min-width: 120px">
            <template #body="{ data }">
              <div class="flex items-center gap-2">
                <span class="font-medium">{{ data.username }}</span>
                <Tag v-if="data.id === authStore.user?.id" value="我" severity="info" size="small" />
              </div>
            </template>
          </Column>
          <Column field="full_name" header="姓名" style="min-width: 100px">
            <template #body="{ data }">
              {{ data.full_name || '-' }}
            </template>
          </Column>
          <Column field="email" header="邮箱" style="min-width: 160px">
            <template #body="{ data }">
              <span class="text-gray-500">{{ data.email || '-' }}</span>
            </template>
          </Column>
          <Column field="role" header="角色" style="width: 110px" alignHeader="center" align="center">
            <template #body="{ data }">
              <Tag :severity="getRoleSeverity(data.role)" :value="getRoleLabel(data.role)" />
            </template>
          </Column>
          <Column field="active" header="状态" style="width: 90px" alignHeader="center" align="center">
            <template #body="{ data }">
              <span :class="['status-badge', data.active ? 'status-active' : 'status-disabled']">
                {{ data.active ? '启用' : '禁用' }}
              </span>
            </template>
          </Column>
          <Column field="last_login_at" header="最后登录" style="min-width: 160px">
            <template #body="{ data }">
              <span class="text-gray-500 text-sm">{{ formatDate(data.last_login_at) }}</span>
            </template>
          </Column>
          <Column header="操作" frozen alignFrozen="right" style="width: 120px">
            <template #body="{ data }">
              <div class="action-buttons">
                <Button v-tooltip.top="'编辑'" icon="pi pi-pencil" class="btn-edit" @click="openEditDialog(data)" />
                <Button 
                  v-tooltip.top="data.id === authStore.user?.id ? '不能删除自己' : (data.role === 'admin' && adminCount <= 1 ? '不能删除最后一个管理员' : '删除')" 
                  icon="pi pi-trash" 
                  class="btn-delete" 
                  :disabled="data.id === authStore.user?.id || (data.role === 'admin' && adminCount <= 1)" 
                  @click="handleDelete(data)" 
                />
              </div>
            </template>
          </Column>
        </DataTable>
      </template>
    </Card>

    <Dialog
      v-model:visible="editDialogVisible"
      :header="isEdit ? '编辑用户' : '新建用户'"
      :style="{ width: '480px' }"
      :modal="true"
    >
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">用户名</label>
          <InputText v-model="editUser!.username" class="w-full" :disabled="isEdit" placeholder="请输入用户名" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">
            密码 <span v-if="isEdit" class="text-gray-400 font-normal">（留空则不修改）</span>
          </label>
          <InputText v-model="editUser!.password" type="password" class="w-full" :placeholder="isEdit ? '留空不修改' : '请输入密码'" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">邮箱</label>
          <InputText v-model="editUser!.email" class="w-full" placeholder="可选" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">姓名</label>
          <InputText v-model="editUser!.full_name" class="w-full" placeholder="可选" />
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">角色</label>
          <Select v-model="editUser!.role" :options="roleOptions" optionLabel="label" optionValue="value" class="w-full" placeholder="选择角色" />
        </div>
        <div v-if="isEdit" class="flex items-center gap-3">
          <label class="text-sm font-medium text-gray-700">启用状态</label>
          <InputToggle v-model="editUser!.active" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <Button label="取消" severity="secondary" @click="editDialogVisible = false" />
          <Button :label="isEdit ? '保存' : '创建'" :loading="editLoading" @click="handleSave" />
        </div>
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1rem;
}

.stat-card :deep(.p-card-content) {
  padding: 1rem;
}

.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-badge {
  display: inline-flex;
  padding: 2px 10px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
}

.status-active {
  background: #ecfdf5;
  color: #059669;
}

.status-disabled {
  background: #fef2f2;
  color: #dc2626;
}

.action-buttons {
  display: flex;
  gap: 4px;
}

.action-buttons :deep(.p-button) {
  width: 32px;
  height: 32px;
  padding: 0;
}

.btn-edit {
  background: transparent !important;
  color: #6b7280 !important;
  border: none !important;
}

.btn-edit:hover {
  color: #3b82f6 !important;
  background: #eff6ff !important;
}

.btn-delete {
  background: transparent !important;
  color: #6b7280 !important;
  border: none !important;
}

.btn-delete:hover {
  color: #ef4444 !important;
  background: #fef2f2 !important;
}

.table-card :deep(.p-card-content) {
  padding: 1.25rem;
}
</style>
