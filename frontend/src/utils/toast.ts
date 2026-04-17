import ToastEventBus from 'primevue/toasteventbus'

export function showToast(options: { severity: string; summary: string; detail?: string; life?: number }) {
    ToastEventBus.emit('add', options)
}
