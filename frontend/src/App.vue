<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { RouterView } from 'vue-router'
import { useWebSocketStore } from '@/stores/websocket'
import { toastEmitter } from '@/api/request'
import { useToast } from 'primevue/usetoast'
import Toast from 'primevue/toast'
import ConfirmDialog from 'primevue/confirmdialog'

const wsStore = useWebSocketStore()
const toast = useToast()

const handleToastEvent = ((e: CustomEvent) => {
    toast.add(e.detail)
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
  <Toast position="top-right" class="toast-top-corner" />
  <ConfirmDialog />
  <RouterView />
</template>
