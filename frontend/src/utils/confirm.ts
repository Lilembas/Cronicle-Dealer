import ConfirmationEventBus from 'primevue/confirmationeventbus'

export interface ConfirmOptions {
  message: string
  header?: string
  icon?: string
  acceptLabel?: string
  rejectLabel?: string
  acceptProps?: Record<string, any>
  rejectProps?: Record<string, any>
  accept?: () => void
  reject?: () => void
}

// 高亮变量内容的 HTML 标签
export function hl(html: string) {
  return `<span class="confirm-highlight">${html}</span>`
}

export function showConfirm(options: ConfirmOptions) {
  ConfirmationEventBus.emit('confirm', options)
}