import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

interface User {
    username: string
    role: string
    fullName: string
}

export const useAuthStore = defineStore('auth', () => {
    const token = ref<string | null>(localStorage.getItem('auth_token'))
    const user = ref<User | null>(null)

    const isAuthenticated = computed(() => !!token.value)
    const isAdmin = computed(() => user.value?.role === 'admin')

    function setToken(newToken: string) {
        token.value = newToken
        localStorage.setItem('auth_token', newToken)
    }

    function setUser(newUser: User) {
        user.value = newUser
    }

    function logout() {
        token.value = null
        user.value = null
        localStorage.removeItem('auth_token')
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
