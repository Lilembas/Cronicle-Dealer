import { defineStore } from 'pinia'
import { ref } from 'vue'
import { statsApi } from '@/api'

export const useSystemStore = defineStore('system', () => {
  const serverTimeOffset = ref(0)
  const currentTime = ref(Date.now())
  const isSynced = ref(false)
  let timer: any = null

  const syncTime = async () => {
    try {
      const stats = await statsApi.get() as any
      if (stats?.server_time) {
        serverTimeOffset.value = stats.server_time - Date.now()
        isSynced.value = true
        // 立即校正一次
        currentTime.value = Date.now() + serverTimeOffset.value
      }
    } catch (error) {
      console.error('Failed to sync time:', error)
    }
  }

  const startClock = () => {
    if (timer) return
    timer = setInterval(() => {
      currentTime.value = Date.now() + serverTimeOffset.value
    }, 1000)
  }

  const stopClock = () => {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  return {
    serverTimeOffset,
    currentTime,
    isSynced,
    syncTime,
    startClock,
    stopClock
  }
})
