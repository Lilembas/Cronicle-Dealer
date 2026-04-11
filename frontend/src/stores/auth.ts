import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

interface User {
    id?: string
    username: string
    role: string
    fullName: string
}

export const useAuthStore = defineStore('auth', () => {
    const token = ref<string | null>(localStorage.getItem('auth_token'))
    const loadUser = (): User | null => {
        try {
            return JSON.parse(localStorage.getItem('auth_user') || 'null')
        } catch {
            return null
        }
    }
    const user = ref<User | null>(loadUser())

    const isAuthenticated = computed(() => !!token.value)
    const isAdmin = computed(() => user.value?.role === 'admin')

    function setToken(newToken: string) {
        token.value = newToken
        localStorage.setItem('auth_token', newToken)
    }

    function setUser(newUser: User) {
        user.value = newUser
        localStorage.setItem('auth_user', JSON.stringify(newUser))
    }

    function logout() {
        token.value = null
        user.value = null
        localStorage.removeItem('auth_token')
        localStorage.removeItem('auth_user')
    }

    return {
        token,
        user,
        isAuthenticated,
        isAdmin,
        setToken,
        setUser,
        logout,
    }
})
