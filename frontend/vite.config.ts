import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import fs from 'fs'
import yaml from 'js-yaml'

// 读取后端配置
const getBackendConfig = () => {
    try {
        const configPath = resolve(__dirname, '../config.yaml')
        const fileContents = fs.readFileSync(configPath, 'utf8')
        return yaml.load(fileContents) as any
    } catch (e) {
        console.warn('无法加载后端配置，使用默认值')
        return {}
    }
}

const backendConfig = getBackendConfig()
const apiPort = backendConfig?.manager?.http_port || 8080
const wsPort = backendConfig?.manager?.websocket_port || 8081
const webHost = backendConfig?.web?.host || '0.0.0.0'
const webPort = backendConfig?.web?.port || 5173

// https://vitejs.dev/config/
export default defineConfig({
    plugins: [vue()],
    resolve: {
        alias: {
            '@': resolve(__dirname, 'src'),
        },
    },
    server: {
        host: webHost,
        port: webPort,
        proxy: {
            '/api': {
                target: `http://localhost:${apiPort}`,
                changeOrigin: true,
            },
            '/ws': {
                target: `ws://localhost:${wsPort}`,
                ws: true,
            },
        },
    },
})
