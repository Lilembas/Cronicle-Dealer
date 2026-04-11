import request from './request'

export interface LoginRequest {
    username: string
    password: string
}

export interface LoginUser {
    id: string
    username: string
    role: string
    full_name: string
}

export interface LoginResponse {
    token: string
    user: LoginUser
}

export const authApi = {
    login: (data: LoginRequest) => request.post<LoginResponse>('/auth/login', data),
    refresh: () => request.post<{ token: string }>('/auth/refresh'),
}
