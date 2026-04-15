import { defineStore } from 'pinia'
import { ref } from 'vue'
import { getWebSocketClient } from '@/utils/websocket'

// 消息处理器类型
type MessageHandler = (data: any) => void

export const useWebSocketStore = defineStore('websocket', () => {
  const wsClient = ref(getWebSocketClient())
  const isConnected = ref(false)
  const isConnecting = ref(false)
  // 消息处理器存储
  const handlers = ref<Map<string, Set<MessageHandler>>>(new Map())
  // 已加入的房间
  const rooms = ref<Set<string>>(new Set())

  // 设置自动重连回调：WebSocketClient 内部的自动重连成功后恢复 handler
  wsClient.value.onReconnect = () => {
    isConnected.value = true
    restoreSubscriptions()
  }

  // 恢复所有处理器和房间到 wsClient（重连后调用）
  function restoreSubscriptions() {
    // 重新注册所有处理器
    handlers.value.forEach((handlerSet, type) => {
      handlerSet.forEach(handler => {
        wsClient.value.onMessage(type, handler)
      })
    })
    // 重新加入所有房间
    rooms.value.forEach(room => {
      wsClient.value.joinRoom(room)
    })
    // 确保在 global 房间
    if (!rooms.value.has('global')) {
      wsClient.value.joinRoom('global')
      rooms.value.add('global')
    }
  }

  // 确保连接并注册所有处理器
  async function ensureConnection() {
    // 如果已连接，直接返回
    if (isConnected.value || wsClient.value.isConnected()) {
      isConnected.value = true
      return
    }
    // 如果正在连接，等待完成
    if (isConnecting.value) {
      // 等待连接完成
      while (isConnecting.value) {
        await new Promise(resolve => setTimeout(resolve, 50))
      }
      // 连接完成后确保 handler 已注册
      if (isConnected.value) {
        restoreSubscriptions()
      }
      return
    }

    isConnecting.value = true
    try {
      await wsClient.value.connect()
      isConnected.value = true
      // 连接成功后恢复所有处理器和房间
      restoreSubscriptions()
    } catch (error) {
      console.error('WebSocket 连接失败:', error)
      isConnected.value = false
    } finally {
      isConnecting.value = false
    }
  }

  // 订阅消息（自动建立连接）
  function onMessage(type: string, handler: MessageHandler) {
    if (!handlers.value.has(type)) {
      handlers.value.set(type, new Set())
    }
    handlers.value.get(type)!.add(handler)

    // 如果已连接，立即注册到 wsClient
    if (isConnected.value && wsClient.value.isConnected()) {
      wsClient.value.onMessage(type, handler)
    } else if (!isConnecting.value) {
      // 未连接且未在连接中，尝试重新连接（handler 会在 restoreSubscriptions 中注册）
      ensureConnection()
    }
    // 如果正在连接中，handler 已存入 Map，connect 完成后会自动注册
  }

  // 取消订阅
  function offMessage(type: string, handler: MessageHandler) {
    const handlerSet = handlers.value.get(type)
    if (handlerSet) {
      handlerSet.delete(handler)
    }
    wsClient.value.offMessage(type, handler)
  }

  // 加入房间
  function joinRoom(room: string) {
    rooms.value.add(room)
    if (isConnected.value && wsClient.value.isConnected()) {
      wsClient.value.joinRoom(room)
    }
  }

  // 离开房间
  function leaveRoom(room: string) {
    rooms.value.delete(room)
    if (isConnected.value && wsClient.value.isConnected()) {
      wsClient.value.leaveRoom(room)
    }
  }

  return {
    wsClient,
    isConnected,
    isConnecting,
    onMessage,
    offMessage,
    joinRoom,
    leaveRoom,
    ensureConnection,
  }
})