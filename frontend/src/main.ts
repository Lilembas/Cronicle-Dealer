import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import Tooltip from 'primevue/tooltip'
import { VueQueryPlugin } from '@tanstack/vue-query'
import CroniclePreset from './theme'

import App from './App.vue'
import router from './router'

import 'primeicons/primeicons.css'
import './styles/index.css'

const app = createApp(App)

// 注册 Pinia
app.use(createPinia())

// 注册路由
app.use(router)

// 注册 PrimeVue + 自定义 Cronicle 主题
app.use(PrimeVue, {
    theme: {
        preset: CroniclePreset,
        options: {
            prefix: 'p',
            darkModeSelector: '.app-dark',
            cssLayer: false
        }
    },
    ripple: true,
    pt: {
        datatable: {
            headerCell: {
                class: 'text-xs font-semibold uppercase tracking-wide text-surface-500'
            }
        }
    }
})

// 注册 Tooltip 指令
app.directive('tooltip', Tooltip)

// 注册 Vue Query
app.use(VueQueryPlugin)

app.mount('#app')
