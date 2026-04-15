<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { useWebSocketStore } from '@/stores/websocket'

const wsStore = useWebSocketStore()

// 应用启动时初始化 WebSocket 连接
onMounted(() => {
  wsStore.ensureConnection()
})
</script>

<template>
  <RouterView />
</template>
