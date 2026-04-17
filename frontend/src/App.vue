<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { RouterView } from 'vue-router'
import { useWebSocketStore } from '@/stores/websocket'
import ToastService from 'primevue/toastservice'
import { toastEmitter } from '@/api/request'
import Toast from 'primevue/toast'
import ConfirmDialog from 'primevue/confirmdialog'

const wsStore = useWebSocketStore()

const handleToastEvent = ((e: CustomEvent) => {
    ToastService.emit('add', e.detail)
}) as EventListener

onMounted(() => {
    wsStore.ensureConnection()
    toastEmitter.addEventListener('toast', handleToastEvent)
})

onUnmounted(() => {
    toastEmitter.removeEventListener('toast', handleToastEvent)
})
</script>

<template>
  <Toast position="top-right" />
  <ConfirmDialog />
  <RouterView />
</template>
