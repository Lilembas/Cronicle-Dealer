import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import Aura from '@primevue/themes/aura'
import Tooltip from 'primevue/tooltip'
import { VueQueryPlugin } from '@tanstack/vue-query'

import App from './App.vue'
import router from './router'

import 'primeicons/primeicons.css'
import './styles/index.css'

const app = createApp(App)

// 注册 Pinia
app.use(createPinia())

// 注册路由
app.use(router)

// 注册 PrimeVue + Aura 主题
app.use(PrimeVue, {
    theme: {
        preset: Aura,
        options: {
            prefix: 'p',
            darkModeSelector: '.app-dark',
            cssLayer: false
        }
    },
    ripple: true
})

// 注册 Tooltip 指令
app.directive('tooltip', Tooltip)

// 注册 Vue Query
app.use(VueQueryPlugin)

app.mount('#app')
