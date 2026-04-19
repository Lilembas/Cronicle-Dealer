import { toastEmitter } from '@/api/request'

export function showToast(options: { severity: string; summary: string; detail?: string; life?: number }) {
    toastEmitter.dispatchEvent(new CustomEvent('toast', { detail: options }))
}
