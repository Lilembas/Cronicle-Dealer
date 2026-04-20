<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import ConfirmationEventBus from 'primevue/confirmationeventbus'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'

const visible = ref(false)
const confirmation = ref<any>(null)

const message = computed(() => confirmation.value?.message || '')
const header = computed(() => confirmation.value?.header || '')
const acceptLabel = computed(() => confirmation.value?.acceptLabel || confirmation.value?.acceptProps?.label || 'OK')
const rejectLabel = computed(() => confirmation.value?.rejectLabel || confirmation.value?.rejectProps?.label || '取消')
const isDanger = computed(() => confirmation.value?.acceptProps?.severity === 'danger')

onMounted(() => {
  ConfirmationEventBus.on('confirm', (options: any) => {
    if (!options) return
    confirmation.value = options
    confirmation.value?.onShow?.()
    visible.value = true
  })
  ConfirmationEventBus.on('close', () => {
    visible.value = false
    confirmation.value = null
  })
})

onUnmounted(() => {
  ConfirmationEventBus.off('confirm', () => {})
  ConfirmationEventBus.off('close', () => {})
})

const accept = () => {
  confirmation.value?.accept?.()
  visible.value = false
}

const reject = () => {
  confirmation.value?.reject?.()
  visible.value = false
}

const onHide = () => {
  confirmation.value?.onHide?.()
  visible.value = false
}
</script>

<template>
  <Dialog
    v-model:visible="visible"
    :modal="true"
    :closable="false"
    :draggable="false"
    :style="{ width: '420px' }"
    :showHeader="true"
    @update:visible="onHide"
  >
    <template #header>
      <div class="confirm-header">
        <i :class="confirmation?.icon || 'pi pi-exclamation-triangle'" class="confirm-header-icon" />
        <span>{{ header }}</span>
      </div>
    </template>
    <div class="confirm-message">
      <span v-html="message" />
    </div>
    <template #footer>
      <Button :label="rejectLabel" severity="secondary" @click="reject" class="confirm-btn" />
      <Button
        :label="acceptLabel"
        :severity="confirmation?.acceptProps?.severity || (isDanger ? 'danger' : 'primary')"
        @click="accept"
        class="confirm-btn"
      />
    </template>
  </Dialog>
</template>

<style scoped>
.confirm-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.confirm-header-icon {
  font-size: 18px;
  color: #f59e0b;
}

.confirm-message {
  padding: 8px 0;
  font-size: 14px;
  line-height: 1.5;
}

.confirm-btn {
  min-width: 80px;
}

:deep(.confirm-highlight) {
  font-family: 'Inter', ui-sans-serif, system-ui, -apple-system, sans-serif;
  color: #3b82f6;
  font-weight: 600;
  padding: 0 2px;
}
</style>
