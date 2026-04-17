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

export function showConfirm(options: ConfirmOptions) {
  ConfirmationEventBus.emit('confirm', options)
}
