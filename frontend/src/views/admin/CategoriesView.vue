<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { adminApi, type AdminCategory } from '@/api'
import { showToast } from '@/utils/toast'
import { showConfirm } from '@/utils/confirm'
import Button from 'primevue/button'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'

const loading = ref(false)
const categories = ref<AdminCategory[]>([])

const editDialogVisible = ref(false)
const editCategory = ref<Partial<AdminCategory> | null>(null)
const editLoading = ref(false)
const isEdit = computed(() => !!editCategory.value?.id)

async function loadCategories() {
  loading.value = true
  try {
    categories.value = (await adminApi.listCategories()) || []
  } catch {
    showToast({ severity: 'error', summary: '加载分组列表失败', life: 5000 })
  } finally {
    loading.value = false
  }
}

function openCreateDialog() {
  editCategory.value = { name: '' }
  editDialogVisible.value = true
}

function openEditDialog(cat: AdminCategory) {
  editCategory.value = { ...cat }
  editDialogVisible.value = true
}

async function handleSave() {
  if (!editCategory.value?.name?.trim()) {
    showToast({ severity: 'warn', summary: '分组名称不能为空', life: 3000 })
    return
  }

  editLoading.value = true
  try {
    if (editCategory.value.id) {
      await adminApi.updateCategory(editCategory.value.id, { name: editCategory.value.name.trim() })
      showToast({ severity: 'success', summary: '分组更新成功', life: 3000 })
    } else {
      await adminApi.createCategory({ name: editCategory.value.name.trim() })
      showToast({ severity: 'success', summary: '分组创建成功', life: 3000 })
    }
    editDialogVisible.value = false
    await loadCategories()
  } catch (error: any) {
    showToast({ severity: 'error', summary: '操作失败', detail: error.response?.data?.error || '请重试', life: 5000 })
  } finally {
    editLoading.value = false
  }
}

async function handleDelete(cat: AdminCategory) {
  if (cat.job_count > 0) {
    showToast({ severity: 'warn', summary: `该分组下有 ${cat.job_count} 个任务，无法删除`, life: 5000 })
    return
  }

  const confirmed = await showConfirm(`确定要删除分组 "${cat.name}" 吗？`, { header: '删除分组' })
  if (!confirmed) return

  try {
    await adminApi.deleteCategory(cat.id)
    showToast({ severity: 'success', summary: '分组已删除', life: 3000 })
    await loadCategories()
  } catch (error: any) {
    showToast({ severity: 'error', summary: '删除失败', detail: error.response?.data?.error || '请重试', life: 5000 })
  }
}

function formatDate(dateStr?: string) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

onMounted(loadCategories)
</script>

<template>
  <div class="categories-page">
    <Card class="table-card">
      <template #content>
        <div class="flex justify-between items-center mb-4">
          <h3 class="text-lg font-semibold text-gray-800">任务分组</h3>
          <Button label="新建分组" icon="pi pi-plus" size="small" @click="openCreateDialog" />
        </div>
        <DataTable :value="categories" stripedRows :loading="loading" dataKey="id">
          <Column field="name" header="分组名称" style="min-width: 200px">
            <template #body="{ data }">
              <span class="font-medium">{{ data.name }}</span>
            </template>
          </Column>
          <Column field="job_count" header="任务数量" style="width: 120px" alignHeader="center" align="center">
            <template #body="{ data }">
              <span :class="data.job_count > 0 ? 'text-blue-500 font-bold' : 'text-gray-400'">
                {{ data.job_count }}
              </span>
            </template>
          </Column>
          <Column field="created_at" header="创建时间" style="min-width: 180px">
            <template #body="{ data }">
              <span class="text-gray-500 text-sm">{{ formatDate(data.created_at) }}</span>
            </template>
          </Column>
          <Column field="updated_at" header="更新时间" style="min-width: 180px">
            <template #body="{ data }">
              <span class="text-gray-500 text-sm">{{ formatDate(data.updated_at) }}</span>
            </template>
          </Column>
          <Column header="操作" frozen alignFrozen="right" style="width: 120px">
            <template #body="{ data }">
              <div class="action-buttons">
                <Button v-tooltip.top="'编辑'" icon="pi pi-pencil" class="btn-edit" @click="openEditDialog(data)" />
                <Button
                  v-tooltip.top="'删除'"
                  icon="pi pi-trash"
                  class="btn-delete"
                  :disabled="data.job_count > 0"
                  @click="handleDelete(data)"
                />
              </div>
            </template>
          </Column>
          <template #empty>
            <div class="text-center py-8 text-gray-400">
              <i class="pi pi-tags text-4xl mb-2 block" />
              <p>暂无分组，点击上方按钮新建</p>
            </div>
          </template>
        </DataTable>
      </template>
    </Card>

    <Dialog
      v-model:visible="editDialogVisible"
      :header="isEdit ? '编辑分组' : '新建分组'"
      :style="{ width: '400px' }"
      :modal="true"
    >
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">分组名称</label>
          <InputText v-model="editCategory!.name" class="w-full" placeholder="请输入分组名称" />
        </div>
        <p v-if="isEdit" class="text-xs text-amber-600">
          注意：修改分组名称后，所有引用该分组的任务将自动更新。
        </p>
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

.btn-delete:hover:not(:disabled) {
  color: #ef4444 !important;
  background: #fef2f2 !important;
}

.btn-delete:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.table-card :deep(.p-card-content) {
  padding: 1.25rem;
}
</style>
