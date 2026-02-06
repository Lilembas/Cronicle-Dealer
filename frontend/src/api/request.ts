import axios from 'axios'
import type { AxiosInstance, AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'

// 创建 axios 实例
const service: AxiosInstance = axios.create({
    baseURL: '/api/v1',
    timeout: 15000,
})

// 请求拦截器
service.interceptors.request.use(
    (config) => {
        const token = localStorage.getItem('auth_token')
        if (token) {
            config.headers.Authorization = `Bearer ${token}`
        }
        return config
    },
    (error) => {
        console.error('请求错误:', error)
        return Promise.reject(error)
    }
)

// 响应拦截器
service.interceptors.response.use(
    (response: AxiosResponse) => response.data,
    (error) => {
        const { response } = error
        const errorMessage = response?.data?.message

        // 错误状态码映射
        const errorMessages: Record<number, string> = {
            401: '未授权，请重新登录',
            403: '拒绝访问',
            404: '请求的资源不存在',
            500: '服务器内部错误',
        }

        const message = errorMessage || errorMessages[response?.status] || '请求失败'
        ElMessage.error(message)

        // 401 时清除 token 并跳转登录
        if (response?.status === 401) {
            localStorage.removeItem('auth_token')
            window.location.href = '/login'
        }

        return Promise.reject(error)
    }
)

export default service
