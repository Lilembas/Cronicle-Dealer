/**
 * WebSocket 客户端封装
 * 支持房间订阅、自动重连、心跳检测
 */

// 客户端消息类型
export interface ClientMessage {
  action: 'join' | 'leave' | 'ping'
  room?: string
  event_id?: string  // 兼容旧写法
}

// 服务器消息类型
export interface ServerMessage {
  type: 'log' | 'task_status' | 'node_status' | 'history_log' | 'error' | 'pong' | 'success'
  data: any
  room?: string
}

// 消息处理器类型
type MessageHandler = (data: any) => void

// WebSocket 配置
interface WebSocketConfig {
  url: string
  reconnectInterval?: number  // 重连间隔（毫秒）
  heartbeatInterval?: number  // 心跳间隔（毫秒）
  debug?: boolean
}

export class WebSocketClient {
  private ws: WebSocket | null = null
  private config: Required<WebSocketConfig>
  private reconnectTimer: number | null = null
  private heartbeatTimer: number | null = null
  private subscriptions: Set<string> = new Set()
  private messageHandlers: Map<string, MessageHandler[]> = new Map()
  private isManualClose: boolean = false
  private retryCount: number = 0
  private maxRetries: number = 10

  constructor(config: WebSocketConfig) {
    this.config = {
      url: config.url,
      reconnectInterval: config.reconnectInterval || 3000,
      heartbeatInterval: config.heartbeatInterval || 30000,
      debug: config.debug || false,
    }
  }

  /**
   * 连接WebSocket
   */
  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(this.config.url)
        this.isManualClose = false

        this.ws.onopen = () => {
          this.log('WebSocket 连接成功')
          this.retryCount = 0
          this.startHeartbeat()
          resolve()
        }

        this.ws.onmessage = (event) => {
          this.handleMessage(event.data)
        }

        this.ws.onerror = (error) => {
          this.log('WebSocket 错误:', error)
          reject(error)
        }

        this.ws.onclose = () => {
          this.log('WebSocket 连接关闭')
          this.stopHeartbeat()

          if (!this.isManualClose) {
            this.scheduleReconnect()
          }
        }
      } catch (error) {
        reject(error)
      }
    })
  }

  /**
   * 断开连接
   */
  disconnect() {
    this.isManualClose = true
    this.stopReconnect()
    this.stopHeartbeat()

    if (this.ws) {
      this.ws.close()
      this.ws = null
    }

    // 清空订阅
    this.subscriptions.clear()
  }

  /**
   * 加入房间
   */
  joinRoom(room: string) {
    if (this.subscriptions.has(room)) {
      this.log(`已经订阅房间: ${room}`)
      return
    }

    this.send({
      action: 'join',
      room,
    })

    this.subscriptions.add(room)
    this.log(`加入房间: ${room}`)
  }

  /**
   * 离开房间
   */
  leaveRoom(room: string) {
    if (!this.subscriptions.has(room)) {
      return
    }

    this.send({
      action: 'leave',
      room,
    })

    this.subscriptions.delete(room)
    this.log(`离开房间: ${room}`)
  }

  /**
   * 订阅任务日志（便捷方法）
   */
  subscribeEventLogs(eventId: string) {
    this.joinRoom(`event:${eventId}`)
  }

  /**
   * 取消订阅任务日志（便捷方法）
   */
  unsubscribeEventLogs(eventId: string) {
    this.leaveRoom(`event:${eventId}`)
  }

  /**
   * 注册消息处理器
   */
  onMessage(type: string, handler: MessageHandler) {
    if (!this.messageHandlers.has(type)) {
      this.messageHandlers.set(type, [])
    }
    this.messageHandlers.get(type)!.push(handler)
  }

  /**
   * 移除消息处理器
   */
  offMessage(type: string, handler: MessageHandler) {
    const handlers = this.messageHandlers.get(type)
    if (handlers) {
      const index = handlers.indexOf(handler)
      if (index > -1) {
        handlers.splice(index, 1)
      }
    }
  }

  /**
   * 发送消息
   */
  private send(message: ClientMessage) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message))
    } else {
      this.log('WebSocket 未连接，无法发送消息')
    }
  }

  /**
   * 处理服务器消息
   */
  private handleMessage(data: string) {
    try {
      const message: ServerMessage = JSON.parse(data)
      this.log('收到消息:', message)

      // 触发对应类型的处理器
      const handlers = this.messageHandlers.get(message.type)
      if (handlers) {
        handlers.forEach(handler => handler(message.data))
      }
    } catch (error) {
      this.log('解析消息失败:', error)
    }
  }

  /**
   * 安排重连
   */
  private scheduleReconnect() {
    if (this.retryCount >= this.maxRetries) {
      this.log('达到最大重试次数，停止重连')
      return
    }

    this.retryCount++
    const delay = this.config.reconnectInterval * Math.pow(1.5, this.retryCount - 1)

    this.log(`${delay}ms 后尝试重连 (${this.retryCount}/${this.maxRetries})...`)

    this.reconnectTimer = window.setTimeout(() => {
      this.connect().catch(() => {
        // 连接失败会自动触发下一次重连
      })
    }, delay)
  }

  /**
   * 停止重连
   */
  private stopReconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  /**
   * 启动心跳
   */
  private startHeartbeat() {
    this.heartbeatTimer = window.setInterval(() => {
      this.send({ action: 'ping' })
    }, this.config.heartbeatInterval)
  }

  /**
   * 停止心跳
   */
  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  /**
   * 日志输出
   */
  private log(...args: any[]) {
    if (this.config.debug) {
      console.log('[WebSocket]', ...args)
    }
  }
}

// 创建全局WebSocket客户端实例
let wsClient: WebSocketClient | null = null

/**
 * 获取WebSocket客户端实例
 */
export function getWebSocketClient(): WebSocketClient {
  if (!wsClient) {
    const wsUrl = `ws://${window.location.hostname}:8082/ws`
    wsClient = new WebSocketClient({
      url: wsUrl,
      debug: import.meta.env.DEV,
    })
  }
  return wsClient
}

/**
 * 初始化WebSocket连接
 */
export async function initWebSocket(): Promise<void> {
  const client = getWebSocketClient()
  if (!client || !client['ws'] || client['ws'].readyState !== WebSocket.OPEN) {
    await client.connect()
  }
}
